#!/bin/sh
set -eu
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/koolshare/bin
export PATH
ROOT=/jffs/asus-domain-query
HOOK=/jffs/scripts/services-start
LINE='/jffs/asus-domain-query/service.sh start >/dev/null 2>&1 & # asus-domain-query'
HERE=$(CDPATH= cd "$(dirname "$0")" && pwd)
fail() { echo "Install refused: $*" >&2; exit 1; }
[ "$(awk '/^Uid:/ {print $2}' /proc/self/status)" = 0 ] || fail 'root/admin required'
[ "$(uname -m)" = aarch64 ] || fail 'this package requires aarch64'
[ "$(nvram get jffs2_scripts)" = 1 ] || fail 'enable JFFS custom scripts first'
[ "$(nvram get lan_ifname)" = br0 ] || fail 'LAN interface must be br0'
[ ! -e "$ROOT" ] && [ ! -L "$ROOT" ] || fail 'already installed; uninstall first'
[ ! -L "$HOOK" ] && [ ! -L /jffs/scripts ] || fail 'symlink startup hook not supported'
[ -d /jffs/scripts ] || fail '/jffs/scripts missing'
[ ! -e "$HOOK.asus-domain-query.tmp" ] && [ ! -L "$HOOK.asus-domain-query.tmp" ] || fail 'temporary hook path already exists'
if [ -e "$HOOK" ]; then
 [ -f "$HOOK" ] || fail 'startup hook is not a regular file'
 [ "$(head -n 1 "$HOOK")" = '#!/bin/sh' ] || fail 'existing startup hook is not a /bin/sh script'
 grep -q asus-domain-query "$HOOK" && fail 'existing query hook needs inspection'
fi
for f in domain-query service.sh uninstall.sh; do [ -f "$HERE/$f" ] || fail "missing $f"; done
(cd "$HERE" && sha256sum -c SHA256SUMS) || fail 'package checksum mismatch'
AVAIL=$(df -k /jffs | awk 'END {print $4}')
NEEDED=$(du -k "$HERE/domain-query" | awk '{print $1+2048}')
[ "$AVAIL" -gt "$NEEDED" ] || fail 'insufficient JFFS space (binary plus 2 MB reserve required)'
"$HERE/domain-query" -lan "$(nvram get lan_ipaddr)" -mask "$(nvram get lan_netmask)" -interface br0 -check || fail 'binary preflight failed'
# mkdir is also the install lock; never replace an existing installation.
umask 077
mkdir "$ROOT"
COMMITTED=0
cleanup() {
 if [ "$COMMITTED" != 1 ]; then
  [ ! -x "$ROOT/service.sh" ] || "$ROOT/service.sh" stop >/dev/null 2>&1 || true
  # Hook is published only after a successful start, immediately before commit.
  rm -f "$HOOK.asus-domain-query.tmp"
  rm -rf "$ROOT"
 fi
}
trap cleanup EXIT
cp "$HERE/domain-query" "$HERE/service.sh" "$HERE/uninstall.sh" "$ROOT/"
chmod 700 "$ROOT/domain-query" "$ROOT/service.sh" "$ROOT/uninstall.sh"
if [ -f "$HOOK" ]; then
 cp -p "$HOOK" "$ROOT/services-start.before"
else
 printf '#!/bin/sh\n' > "$ROOT/services-start.before"
 chmod 755 "$ROOT/services-start.before"
 touch "$ROOT/hook-created"
fi
cp -p "$ROOT/services-start.before" "$HOOK.asus-domain-query.tmp"
awk -v line="$LINE" 'NR==1 {print; print line; next} {print}' "$ROOT/services-start.before" > "$HOOK.asus-domain-query.tmp"
/bin/sh -n "$HOOK.asus-domain-query.tmp"
# Merlin requires an executable hook; retain original mode for uninstall.
chmod u+x "$HOOK.asus-domain-query.tmp"
"$ROOT/service.sh" start
mv "$HOOK.asus-domain-query.tmp" "$HOOK"
COMMITTED=1
printf 'Installed. Open http://%s:18085/ from the LAN.\n' "$(nvram get lan_ipaddr)"
