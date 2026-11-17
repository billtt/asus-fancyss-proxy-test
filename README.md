# ASUS fancyss 域名分流查询

为 ASUS RT-AX86U Pro / aarch64 / Koolshare 固件编写的独立 LAN 查询工具。规则模型面向 fancyss_hnd_v8_lite 3.5.29；程序每次核对实际规则，遇到不支持的规则不作推断。

输入域名后，工具查询路由器 LAN DNS 本次返回的全部 IPv4，逐个检查 ipset，显示预期代理/直连、首个生效规则和全部命中集合。支持软件中心入口卡片。界面无外部资源依赖。

## 部署方式

- 独立 Go 程序，无需在路由器安装 Go、Python 或 Node。
- 文件持久化于 `/jffs/asus-domain-query`。
- HTTP 服务监听路由器 LAN IPv4 的 `18085` 端口，并绑定 `br0`；绑定失败即退出。
- 启动时从 nvram 读取 LAN 地址和掩码，不内置具体部署地址。
- `/jffs/scripts/services-start` 仅增加一行独立后台启动调用，保留原有内容和备份。
- 不写入 fancyss 配置、iptables 或 ipset，不重启路由器、DNS、代理或管理服务。

查询只执行固定程序与参数数组：iptables 的 `-S`、ipset 的 `list -n` 和 `test`。不会通过 shell 执行域名。DNS 查询可能触发 dnsmasq 自身正常更新动态集合，因此“只读”指不主动修改规则、配置或集合。

## 构建

需要 macOS 或 Linux、Go 1.24 或更高版本、Python 3，以及 `sha256sum` 或 `shasum`。race detector 需要 Go 支持的本机构建环境和 C 编译器。Go 依赖仅限标准库。

在项目目录运行：

```sh
./build.sh
# 或指定 Go 可执行文件
GO=/path/to/go ./build.sh
```

脚本执行测试、交叉编译 Linux arm64，并生成：

```text
dist/asus-domain-query-aarch64.tar.gz
```

构建缓存和临时测试目录使用 `work/`。`work/`、`dist/` 均不提交到仓库；公开仓库只包含源代码、通用文档、图标和测试。

## 从一台全新的电脑开始

以下命令适用于 macOS 或 Linux；Windows 可使用 WSL 的 Linux 环境。先准备 Git、Go、Python 3 和 C 编译器。macOS 的开发者命令行工具可提供 Git/C 编译器；Linux 使用发行版的软件包安装这些依赖。

从本公开仓库页面复制 HTTPS 克隆地址，不需要配置 GitHub SSH 密钥：

```sh
git clone <本仓库的HTTPS克隆地址> asus-fancyss-proxy-test
cd asus-fancyss-proxy-test
./build.sh
```

也可从 GitHub 的 **Code → Download ZIP** 获取源码并解压，在解压目录执行 `./build.sh`。本仓库不要求原开发电脑上的缓存、工具链或任何私有文件。

在新路由器后台开启 **仅 LAN 的 SSH** 和 **JFFS 自定义脚本**。确认是支持的 aarch64/Koolshare 环境，且 fancyss 规则与上文模型一致。先用自己的 SSH 账号登录检查；不要为了安装工具修改现有 fancyss 规则。

将以下三项替换为新路由器实际值，然后在电脑的项目目录执行：

```sh
ROUTER_HOST='<新路由器的LAN地址>'
ROUTER_USER='<SSH管理用户名>'
ROUTER_PORT='<SSH端口>'

ssh -p "$ROUTER_PORT" "$ROUTER_USER@$ROUTER_HOST"   'umask 077; mkdir -p /tmp/asus-domain-query-package'
cat dist/asus-domain-query-aarch64.tar.gz |   ssh -p "$ROUTER_PORT" "$ROUTER_USER@$ROUTER_HOST"   'tar -xz -C /tmp/asus-domain-query-package && /bin/sh /tmp/asus-domain-query-package/asus-domain-query/install.sh && /bin/sh /tmp/asus-domain-query-package/asus-domain-query/install-entry.sh'
```

首次 SSH 连接请核对新路由器主机密钥；无需把密码写进命令。看到安装成功后，访问 `http://<新路由器的LAN地址>:18085/`，或刷新管理后台的“软件中心 → 已安装 → 域名分流查询”。不同 LAN 地址会自动从新路由器读取，无需修改源代码。

只做一次状态和健康检查，再查询少量域名即可：

```sh
ssh -p "$ROUTER_PORT" "$ROUTER_USER@$ROUTER_HOST"   '/jffs/asus-domain-query/service.sh status'
curl --noproxy '*' "http://$ROUTER_HOST:18085/health"
```

不需要重启路由器或 fancyss。如果规则不匹配而显示 UNKNOWN，应先核查规则与工具支持范围，不能把它当成网络故障或直接修改生产规则。安装后的停止、撤销入口和完整卸载命令见下文。

## 安装

路由器需满足：aarch64、启用 JFFS 自定义脚本、LAN 接口为 br0、私有 IPv4 和合法子网掩码，且具备 iptables、ipset、nvram、dbus 等固件工具。

使用自己的 SSH 客户端将安装包上传到路由器 `/tmp`，然后在路由器管理员终端执行：

```sh
mkdir -p /tmp/asus-domain-query-package
tar -xz -f /tmp/asus-domain-query-aarch64.tar.gz -C /tmp/asus-domain-query-package
/bin/sh /tmp/asus-domain-query-package/asus-domain-query/install.sh
# 可选：添加软件中心入口
/bin/sh /tmp/asus-domain-query-package/asus-domain-query/install-entry.sh
```

安装程序会打印访问地址：`http://<LAN_IP>:18085/`。从 LAN 客户端打开该地址即可。

安装前验证包内 `SHA256SUMS`、处理器、权限、JFFS 脚本设置、目录冲突、空间、网络参数及程序可运行性。校验和用于可信传输后的完整性检查，不是发行者签名。需要至少“程序大小 + 2 MB”空余 JFFS 空间；程序约 6 MB，实际大小随 Go 版本变化。

安装拒绝覆盖已有部署或符号链接启动文件。启动失败时清理新安装，不发布自启调用。原启动文件备份在 `/jffs/asus-domain-query/services-start.before`。

## 管理后台入口

入口位置：**路由器管理后台 → 软件中心 → 已安装 → 域名分流查询**。

卡片在新标签页打开独立工具。新增的内容仅为自己的 `Module_asusdomainquery.asp`、图标、入口卸载脚本和 `softcenter_module_asusdomainquery_` 六个展示字段；不修改软件中心、菜单或 fancyss 文件。

建议通过路由器 LAN IP 的 HTTP 管理地址进入。工具允许同站、不同端口的顶层 GET 导航到静态首页，查询接口仍要求同源。使用其他管理域名或 HTTPS 时，浏览器可能将跳转判定为跨站并拒绝；此时使用 LAN IP 管理地址或直接打开查询地址。

卡片的“卸载”只移除入口，不停止查询服务。也可在路由器终端单独撤销入口：

```sh
/jffs/asus-domain-query/remove-entry.sh
```

## 状态、停止与卸载

在路由器终端执行：

```sh
/jffs/asus-domain-query/service.sh status
/jffs/asus-domain-query/service.sh stop
/jffs/asus-domain-query/service.sh start
# 卸载整个工具及入口
/jffs/asus-domain-query/uninstall.sh
```

仅终止 PID 与 `/proc/PID/exe` 同时匹配本程序的进程。卸载只删除自己的精确启动行，保留其他工具后续修改；没有其他修改时恢复原文件及权限。升级采用先卸载、后安装。

如 LAN 地址改变，停止并重新启动本工具以读取新配置；浏览器也应使用新地址。程序不会自动跟踪接口重配置。服务在开机时通过 `services-start` 启动。不要为了测试此工具而重启生产路由器。

健康检查路径为 `/health`，查询接口为 `POST /api/query`，请求格式：

```json
{"domain":"example.com"}
```

请求头使用 `Content-Type: application/json`。从 LAN 客户端测试；服务严格绑定 br0，不监听 loopback。

## 判定规则和边界

仅接受下面两条完整链的受支持顺序：

| 顺序 | 条件 | 预期动作 |
|---|---|---|
| 1 | SHADOWSOCKS：命中 ignlist | RETURN / 预期直连 |
| 2 | TCP 22、80、443、8080、8443 | 进入 SHADOWSOCKS_CHN |
| 3 | 命中 black_list | REDIRECT :3333 / 预期代理 |
| 4 | 命中 chnlist | RETURN |
| 5 | 命中 chnroute | RETURN |
| 6 | 命中 white_list | RETURN |
| 7 | 未命中 | REDIRECT :3333 / 预期代理 |

链缺失、规则顺序或目标变化、附加规则、集合缺失、命令错误以及查询前后规则变化，都保守返回 UNKNOWN。错误不会被当成未命中。动态 ipset 不是原子快照。

结果是**假设流量已进入 SHADOWSOCKS 链，且使用上述 TCP 端口**的条件性预测。未模拟前置 PREROUTING、设备 ACL、源地址策略、其他 NAT 链、策略路由、路由器 OUTPUT、已有 conntrack、IPv6 和 UDP/QUIC。RETURN 只是离开当前链，后续规则可能继续改变路径。PROXY 不证明节点可用或真实连接成功。

DNS 使用路由器 LAN DNS，只查询 A 记录并跟随 CNAME，去重且不截断。结果是本次 DNS 响应，不是域名全球/CDN 的所有地址。超过 128 个 IPv4、无 A 记录、DNS 超时均明确报错。浏览器 DoH、缓存或 CDN 差异可能造成实际访问差异。

## 访问限制

- 同一受信任 LAN 子网内任何设备均可查询，没有独立用户登录。
- 严格校验 LAN 来源、Host、Origin 和 Fetch Metadata；不启用 CORS。
- 拒绝 URL、IP literal、Unicode、单标签名称和 shell 字符；国际化域名使用 punycode。
- 一次只允许一个查询，请求间隔至少一秒。
- DNS 超时 5 秒、单命令 2 秒、全查询 18 秒，请求体上限 1 KB。
- GET 不执行检测；前端使用 textContent 显示结果。

不要将此工具转发到公网、放在不受信任共享网络或通过反向代理扩大访问范围。

## 验证与隐私

测试覆盖域名校验、规则优先级、DNS 多地址、失败路径、HTTP 边界，以及隔离目录中的安装回滚和卸载。详见 `VERIFICATION.md`。

提交时不要包含设备导出、真实网络地址、SSH 账号/端口、凭据、浏览器会话、实机日志、备份或个人路径。测试中的私有网段为虚构测试数据；公共目标地址使用文档示例地址。

## 研究来源

- [fancyss 页面源码](https://github.com/hq450/fancyss/blob/3.0/fancyss/webs/Module_shadowsocks.asp)
- [fancyss 后端调用示例](https://github.com/hq450/fancyss/blob/3.0/fancyss/scripts/ss_config.sh)
- [Koolshare base.sh](https://github.com/koolshare/rogsoft/blob/master/softcenter/softcenter/scripts/base.sh)
- [Merlin JFFS 用户脚本](https://github.com/Asuswrt-Merlin/asuswrt-merlin.ng/wiki/User-scripts)
- [Go DNS resolver](https://pkg.go.dev/net#Resolver.LookupIP)

公开上游分支可能比目标版本新；程序以实际规则校验为准，不假设所有 fancyss 版本兼容。
