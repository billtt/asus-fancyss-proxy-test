package main

import (
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func TestDomainValidation(t *testing.T) {
	for _, s := range []string{"github.com", "GitHub.COM.", "xn--fiqs8s.example", "a-b.example"} {
		if _, e := domainName(s); e != nil {
			t.Errorf("valid %q: %v", s, e)
		}
	}
	for _, s := range []string{"", "github.com;id", "$(id).com", "`id`.com", "a.com\n", "-a.com", "a-.com", "a..com", "https://a.com", "a.com/path", "127.0.0.1", "::1", "1.2.3.999", "localhost", "a_b.com", "中文.com", strings.Repeat("a", 64) + ".com", "a.com\x00", " a.com", "a.com..", strings.Repeat("a.", 127) + "com"} {
		if _, e := domainName(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func miss() (string, error) {
	return "203.0.113.8 is NOT in set x.", exec.Command("sh", "-c", "exit 1").Run()
}
func fake(hits map[string]bool) backend {
	return backend{resolve: func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.8"), net.ParseIP("203.0.113.8"), net.ParseIP("::1")}, nil
	}, run: func(_ context.Context, name string, args ...string) (string, error) {
		if name == "iptables" {
			if args[3] == "SHADOWSOCKS" {
				return parentRules, nil
			}
			return childRules, nil
		}
		if args[0] == "list" {
			return strings.Join(sets, "\n"), nil
		}
		if hits[args[1]] {
			return "in set", nil
		}
		return miss()
	}}
}
func TestPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, want, rule string
		hits             map[string]bool
	}{
		{"default", "PROXY", "default → REDIRECT :3333", nil},
		{"ignore overrides black", "DIRECT", "ignlist", map[string]bool{"ignlist": true, "black_list": true}},
		{"black overrides china", "PROXY", "black_list", map[string]bool{"black_list": true, "chnroute": true}},
		{"china domain", "DIRECT", "chnlist", map[string]bool{"chnlist": true, "chnroute": true}},
		{"china route", "DIRECT", "chnroute", map[string]bool{"chnroute": true, "white_list": true}},
		{"whitelist", "DIRECT", "white_list", map[string]bool{"white_list": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := fake(tc.hits).query(context.Background(), "github.com", "10.23.45.1")
			if e != nil || len(r.Rows) != 1 || r.Rows[0].Decision != tc.want || r.Rows[0].Rule != tc.rule || !r.Verified {
				t.Fatalf("%+v %v", r, e)
			}
		})
	}
}
func TestFailuresReturnUnknown(t *testing.T) {
	for _, mode := range []string{"missing-set", "changed-rule", "command-failure", "rule-race"} {
		t.Run(mode, func(t *testing.T) {
			b := fake(nil)
			orig := b.run
			count := 0
			b.run = func(ctx context.Context, n string, a ...string) (string, error) {
				if mode == "missing-set" && n == "ipset" && a[0] == "list" {
					return "ignlist", nil
				}
				if mode == "changed-rule" && n == "iptables" {
					return strings.ReplaceAll(childRules, "3333", "9999"), nil
				}
				if mode == "command-failure" && n == "ipset" && a[0] == "test" {
					return "permission denied", errors.New("bad")
				}
				if mode == "rule-race" && n == "iptables" {
					count++
					if count > 2 {
						return "changed", nil
					}
				}
				return orig(ctx, n, a...)
			}
			r, e := b.query(context.Background(), "github.com", "10.23.45.1")
			if e != nil || r.Rows[0].Decision != "UNKNOWN" || r.Warning == "" {
				t.Fatalf("%+v %v", r, e)
			}
		})
	}
}
func TestAllDNSAddresses(t *testing.T) {
	b := fake(nil)
	b.resolve = func(context.Context, string) ([]net.IP, error) {
		var out []net.IP
		for i := 1; i <= 8; i++ {
			out = append(out, net.IPv4(198, 51, 100, byte(i)))
		}
		return out, nil
	}
	r, e := b.query(context.Background(), "github-cloud.s3.amazonaws.com", "10.23.45.1")
	if e != nil || len(r.Rows) != 8 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestDNSErrors(t *testing.T) {
	for _, mode := range []string{"timeout", "empty", "too-many"} {
		t.Run(mode, func(t *testing.T) {
			b := fake(nil)
			b.resolve = func(context.Context, string) ([]net.IP, error) {
				if mode == "timeout" {
					return nil, context.DeadlineExceeded
				}
				if mode == "empty" {
					return nil, nil
				}
				var ips []net.IP
				for i := 0; i < 129; i++ {
					ips = append(ips, net.IPv4(198, 51, 100, byte(i)))
				}
				return ips, nil
			}
			if _, e := b.query(context.Background(), "a.com", "10.23.45.1"); e == nil {
				t.Fatal("expected DNS failure")
			}
		})
	}
}
func TestHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, remote, host, origin, site, method, typ, body string
		status                                              int
	}{
		{"ok", "10.23.45.20:1000", "10.23.45.1:18085", "", "", "POST", "application/json", `{"domain":"github.com"}`, 200},
		{"WAN", "203.0.113.9:1000", "10.23.45.1:18085", "", "", "POST", "application/json", `{"domain":"github.com"}`, 403},
		{"guest", "10.23.46.2:1000", "10.23.45.1:18085", "", "", "POST", "application/json", `{}`, 403},
		{"rebinding", "10.23.45.20:1000", "evil.example:18085", "", "", "POST", "application/json", `{}`, 403},
		{"origin", "10.23.45.20:1000", "10.23.45.1:18085", "https://evil.example", "", "POST", "application/json", `{}`, 403},
		{"fetchsite", "10.23.45.20:1000", "10.23.45.1:18085", "", "cross-site", "POST", "application/json", `{}`, 403},
		{"GET query", "10.23.45.20:1000", "10.23.45.1:18085", "", "", "GET", "application/json", `{}`, 405},
		{"form", "10.23.45.20:1000", "10.23.45.1:18085", "", "", "POST", "text/plain", `{}`, 415},
		{"extra-field", "10.23.45.20:1000", "10.23.45.1:18085", "", "", "POST", "application/json", `{"domain":"github.com","cmd":"id"}`, 400},
		{"injection", "10.23.45.20:1000", "10.23.45.1:18085", "", "", "POST", "application/json", `{"domain":"github.com;id"}`, 400},
		{"trailing", "10.23.45.20:1000", "10.23.45.1:18085", "", "", "POST", "application/json", `{"domain":"github.com"}{}`, 400},
		{"large", "10.23.45.20:1000", "10.23.45.1:18085", "", "", "POST", "application/json", strings.Repeat("x", 1100), 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, _ := networkFor("10.23.45.1", "255.255.255.0")
			a := &app{b: fake(nil), host: "10.23.45.1:18085", network: n}
			r := httptest.NewRequest(tc.method, "http://"+tc.host+"/api/query", strings.NewReader(tc.body))
			r.RemoteAddr = tc.remote
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			r.Header.Set("Content-Type", tc.typ)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestNetworkMask(t *testing.T) {
	for _, mask := range []string{"0.0.0.0", "255.0.255.0", "255.255.255.255"} {
		if _, e := networkFor("10.23.45.1", mask); e == nil {
			t.Fatal(mask)
		}
	}
}
func TestBusyAndStatic(t *testing.T) {
	n, _ := networkFor("10.23.45.1", "255.255.255.0")
	a := &app{b: fake(nil), host: "10.23.45.1:18085", network: n, busy: true}
	for path, status := range map[string]int{"/": 200, "/app.js": 200, "/style.css": 200, "/health": 200, "/../main.go": 404} {
		r := httptest.NewRequest("GET", "http://10.23.45.1:18085"+path, nil)
		r.RemoteAddr = "10.23.45.2:1000"
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://10.23.45.1:18085/api/query", strings.NewReader(`{"domain":"github.com"}`))
	r.RemoteAddr = "10.23.45.2:1000"
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
}

func TestAdminNavigation(t *testing.T) {
	n, _ := networkFor("10.23.45.1", "255.255.255.0")
	a := &app{b: fake(nil), host: "10.23.45.1:18085", network: n}
	for _, tc := range []struct {
		path, site, mode, dest, method string
		want                           int
	}{
		{"/", "same-site", "navigate", "document", "GET", 200},
		{"/", "cross-site", "navigate", "document", "GET", 403},
		{"/", "same-site", "cors", "empty", "GET", 403},
		{"/api/query", "same-site", "navigate", "document", "POST", 403},
		{"/api/query", "same-site", "navigate", "document", "GET", 403},
		{"/app.js", "same-site", "navigate", "document", "GET", 403},
	} {
		r := httptest.NewRequest(tc.method, "http://10.23.45.1:18085"+tc.path, nil)
		r.RemoteAddr = "10.23.45.20:12345"
		r.Header.Set("Sec-Fetch-Site", tc.site)
		r.Header.Set("Sec-Fetch-Mode", tc.mode)
		r.Header.Set("Sec-Fetch-Dest", tc.dest)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%+v got %d", tc, w.Code)
		}
	}
}
