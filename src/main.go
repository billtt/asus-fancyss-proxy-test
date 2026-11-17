package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var web embed.FS
var sets = []string{"ignlist", "black_list", "chnlist", "chnroute", "white_list"}

const parentRules = `-N SHADOWSOCKS
-A SHADOWSOCKS -p tcp -m set --match-set ignlist dst -j RETURN
-A SHADOWSOCKS -p tcp -m multiport --dports 22,80,443,8080,8443 -j SHADOWSOCKS_CHN`
const childRules = `-N SHADOWSOCKS_CHN
-A SHADOWSOCKS_CHN -p tcp -m set --match-set black_list dst -j REDIRECT --to-ports 3333
-A SHADOWSOCKS_CHN -p tcp -m set --match-set chnlist dst -j RETURN
-A SHADOWSOCKS_CHN -p tcp -m set --match-set chnroute dst -j RETURN
-A SHADOWSOCKS_CHN -p tcp -m set --match-set white_list dst -j RETURN
-A SHADOWSOCKS_CHN -p tcp -j REDIRECT --to-ports 3333`

type command func(context.Context, string, ...string) (string, error)
type lookup func(context.Context, string) ([]net.IP, error)
type backend struct {
	run     command
	resolve lookup
}
type row struct {
	IP       string            `json:"ip"`
	Decision string            `json:"decision"`
	Rule     string            `json:"rule"`
	Sets     map[string]string `json:"sets"`
}
type result struct {
	Domain   string `json:"domain"`
	DNS      string `json:"dns"`
	Time     string `json:"time"`
	Verified bool   `json:"verified"`
	Warning  string `json:"warning"`
	Rows     []row  `json:"rows"`
}

func domainName(s string) (string, error) {
	// No shell, URL, IP literals, Unicode, whitespace, or ambiguous numeric hostnames.
	s = strings.ToLower(strings.TrimSuffix(s, "."))
	if len(s) < 1 || len(s) > 253 || net.ParseIP(s) != nil || !strings.Contains(s, ".") {
		return "", errors.New("请输入完整域名；不接受 URL、IP 或单标签名称")
	}
	numeric := true
	for _, c := range s {
		if (c < '0' || c > '9') && c != '.' {
			numeric = false
		}
	}
	if numeric {
		return "", errors.New("不接受数字 IP 格式")
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("域名标签格式不合法")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", errors.New("仅接受 ASCII 域名；国际化域名请使用 punycode")
			}
		}
	}
	return s, nil
}
func canonical(s string) string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		out = append(out, strings.Join(strings.Fields(l), " "))
	}
	return strings.Join(out, "\n")
}
func (b backend) rules(ctx context.Context) (string, bool) {
	p, e := b.run(ctx, "iptables", "-t", "nat", "-S", "SHADOWSOCKS")
	c, f := b.run(ctx, "iptables", "-t", "nat", "-S", "SHADOWSOCKS_CHN")
	return p + "\n" + c, e == nil && f == nil && canonical(p) == parentRules && canonical(c) == childRules
}
func (b backend) query(ctx context.Context, d, dns string) (result, error) {
	r := result{Domain: d, DNS: dns, Time: time.Now().UTC().Format(time.RFC3339), Rows: []row{}}
	ips, err := b.resolve(ctx, d+".")
	if err != nil {
		return r, errors.New("DNS 查询失败、超时或没有 A 记录")
	}
	unique := map[string]bool{}
	for _, ip := range ips {
		if ip.To4() != nil {
			unique[ip.String()] = true
		}
	}
	if len(unique) == 0 {
		return r, errors.New("本次 DNS 响应没有 IPv4 A 记录（可能仅有 IPv6）")
	}
	if len(unique) > 128 {
		return r, errors.New("DNS 返回超过 128 个 IPv4；为保护路由器，本次未执行检测，未返回截断结果")
	}
	keys := make([]string, 0, len(unique))
	for ip := range unique {
		keys = append(keys, ip)
	}
	sort.Strings(keys)
	snapshot, valid := b.rules(ctx)
	names, e := b.run(ctx, "ipset", "list", "-n")
	present := map[string]bool{}
	for _, name := range strings.Fields(names) {
		present[name] = true
	}
	if e != nil {
		valid = false
	}
	for _, set := range sets {
		if !present[set] {
			valid = false
		}
	}
	for _, ip := range keys {
		item := row{IP: ip, Decision: "UNKNOWN", Rule: "无法验证", Sets: map[string]string{}}
		healthy := true
		for _, set := range sets {
			state := "ERROR"
			if present[set] {
				output, e := b.run(ctx, "ipset", "test", set, ip)
				if e == nil {
					state = "HIT"
				} else {
					var x *exec.ExitError
					if errors.As(e, &x) && x.ExitCode() == 1 && strings.Contains(strings.ToLower(output), "is not in set") {
						state = "MISS"
					}
				}
			}
			item.Sets[set] = state
			if state == "ERROR" {
				healthy = false
			}
		}
		if valid && healthy {
			item.Decision = "PROXY"
			item.Rule = "default → REDIRECT :3333"
			for _, set := range sets {
				if item.Sets[set] == "HIT" {
					item.Rule = set
					if set == "black_list" {
						item.Decision = "PROXY"
					} else {
						item.Decision = "DIRECT"
					}
					break
				}
			}
		}
		r.Rows = append(r.Rows, item)
	}
	after, stable := b.rules(ctx)
	r.Verified = valid && stable && snapshot == after
	if !r.Verified {
		r.Warning = "当前规则缺失、与受支持顺序不符，或查询期间发生变化；不作代理/直连判断。"
		for i := range r.Rows {
			r.Rows[i].Decision = "UNKNOWN"
			r.Rows[i].Rule = "规则未验证"
		}
	}
	for _, item := range r.Rows {
		if item.Decision == "UNKNOWN" && r.Warning == "" {
			r.Warning = "部分 ipset 检测失败；对应地址标记为无法判断。"
		}
	}
	if ctx.Err() != nil {
		return r, errors.New("检测超过时间限制，请稍后重试")
	}
	return r, nil
}

// Only fixed program names and argument arrays; never a shell or eval.
func systemRunner() (command, error) {
	paths := map[string]string{}
	for _, name := range []string{"iptables", "ipset"} {
		p, e := exec.LookPath(name)
		if e != nil {
			return nil, e
		}
		paths[name] = p
	}
	return func(ctx context.Context, name string, args ...string) (string, error) {
		p, ok := paths[name]
		if !ok {
			return "", errors.New("unsupported command")
		}
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(c, p, args...)
		cmd.Env = []string{"PATH=/bin:/usr/bin:/sbin:/usr/sbin:/koolshare/bin", "LC_ALL=C"}
		out, e := cmd.CombinedOutput()
		return string(out), e
	}, nil
}

type app struct {
	b         backend
	host, dns string
	network   *net.IPNet
	mu        sync.Mutex
	busy      bool
	last      time.Time
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	remote, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil || !a.network.Contains(net.ParseIP(remote)) || r.Host != a.host {
		http.Error(w, "LAN access only", http.StatusForbidden)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+a.host {
		http.Error(w, "Invalid origin", 403)
		return
	}
	// The admin page lives on another port: allow same-site top-level navigation
	// to the static home page only. API requests retain the strict boundary.
	adminNavigation := r.Method == "GET" && r.URL.Path == "/" &&
		r.Header.Get("Sec-Fetch-Site") == "same-site" &&
		r.Header.Get("Sec-Fetch-Mode") == "navigate" &&
		r.Header.Get("Sec-Fetch-Dest") == "document"
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" && !adminNavigation {
		http.Error(w, "Cross-site request denied", 403)
		return
	}
	if r.URL.Path == "/api/query" {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "JSON required", 415)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		defer r.Body.Close()
		var input struct {
			Domain string `json:"domain"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if dec.Decode(&input) != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			http.Error(w, "Invalid JSON", 400)
			return
		}
		d, e := domainName(input.Domain)
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		a.mu.Lock()
		if a.busy || time.Since(a.last) < time.Second {
			a.mu.Unlock()
			http.Error(w, "查询繁忙，请稍后重试", 429)
			return
		}
		a.busy = true
		a.last = time.Now()
		a.mu.Unlock()
		defer func() { a.mu.Lock(); a.busy = false; a.mu.Unlock() }()
		ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
		defer cancel()
		out, e := a.b.query(ctx, d, a.dns)
		if e != nil {
			http.Error(w, e.Error(), 502)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(out)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "GET required", 405)
		return
	}
	file, typ := "", ""
	switch r.URL.Path {
	case "/":
		file = "index.html"
		typ = "text/html; charset=utf-8"
	case "/app.js":
		file = "app.js"
		typ = "text/javascript; charset=utf-8"
	case "/style.css":
		file = "style.css"
		typ = "text/css; charset=utf-8"
	case "/health":
		w.Header().Set("Content-Type", "text/plain")
		if r.Method != "HEAD" {
			io.WriteString(w, "ok\n")
		}
		return
	default:
		http.NotFound(w, r)
		return
	}
	data, _ := web.ReadFile("web/" + file)
	w.Header().Set("Content-Type", typ)
	if r.Method != "HEAD" {
		w.Write(data)
	}
}
func networkFor(ip, mask string) (*net.IPNet, error) {
	addr := net.ParseIP(ip).To4()
	m := net.ParseIP(mask).To4()
	if addr == nil || m == nil || !addr.IsPrivate() {
		return nil, errors.New("LAN IPv4 must be private")
	}
	ones, bits := net.IPMask(m).Size()
	if bits != 32 || ones < 8 || ones > 30 {
		return nil, errors.New("invalid LAN netmask")
	}
	return &net.IPNet{IP: addr.Mask(net.IPMask(m)), Mask: net.IPMask(m)}, nil
}
func main() {
	lan := flag.String("lan", "", "LAN IPv4 (required; service.sh reads nvram)")
	mask := flag.String("mask", "255.255.255.0", "LAN netmask")
	iface := flag.String("interface", "br0", "LAN interface")
	port := flag.Int("port", 18085, "LAN HTTP port")
	check := flag.Bool("check", false, "validate startup prerequisites without listening")
	flag.Parse()
	network, e := networkFor(*lan, *mask)
	if e != nil {
		log.Fatal(e)
	}
	if *port < 1024 || *port > 65535 {
		log.Fatal("invalid port")
	}
	device, e := net.InterfaceByName(*iface)
	if e != nil {
		log.Fatal(e)
	}
	addrs, e := device.Addrs()
	if e != nil {
		log.Fatal(e)
	}
	found := false
	for _, a := range addrs {
		ip, _, _ := net.ParseCIDR(a.String())
		if ip != nil && ip.String() == *lan {
			found = true
		}
	}
	if !found {
		log.Fatal("LAN address is not assigned to requested interface")
	}
	os.Setenv("PATH", "/bin:/usr/bin:/sbin:/usr/sbin:/koolshare/bin")
	run, e := systemRunner()
	if e != nil {
		log.Fatal(e)
	}
	if *check {
		fmt.Println("preflight ok")
		return
	}
	resolver := &net.Resolver{PreferGo: true, StrictErrors: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 4 * time.Second}
		return d.DialContext(ctx, network, net.JoinHostPort(*lan, "53"))
	}}
	b := backend{run: run, resolve: func(ctx context.Context, domain string) ([]net.IP, error) {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return resolver.LookupIP(c, "ip4", domain)
	}}
	host := net.JoinHostPort(*lan, fmt.Sprint(*port))
	a := &app{b: b, host: host, dns: *lan, network: network}
	listener, e := listenLAN(context.Background(), host, *iface)
	if e != nil {
		log.Fatal(e)
	}
	log.Printf("LAN domain query on http://%s (%s)", host, *iface)
	server := &http.Server{Handler: a, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 22 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 4096}
	log.Fatal(server.Serve(listener))
}
