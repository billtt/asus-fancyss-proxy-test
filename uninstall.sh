#!/bin/sh
set -eu
ROOT=/jffs/asus-domain-query
HOOK=/jffs/scripts/services-start
LINE='/jffs/asus-domain-query/service.sh start >/dev/null 2>&1 & # asus-domain-query'
[ "$(awk '/^Uid:/ {print $2}' /proc/self/status)" = 0 ] || { echo 'root/admin required'; exit 1; }
[ -d "$ROOT" ] && [ ! -L "$ROOT" ] || { echo 'Not installed or unsafe path'; exit 1; }
[ ! -L "$HOOK" ] || { echo 'Refusing to edit symlink hook'; exit 1; }
[ ! -e "$HOOK.asus-domain-query.tmp" ] && [ ! -L "$HOOK.asus-domain-query.tmp" ] || { echo 'Temporary hook path exists'; exit 1; }
[ ! -f "$ROOT/entry-installed" ] || "$ROOT/remove-entry.sh"
"$ROOT/service.sh" stop
if [ -f "$HOOK" ]; then
 # Remove only our exact line, retaining any changes other tools made afterwards.
 cp -p "$HOOK" "$HOOK.asus-domain-query.tmp"
 awk -v line="$LINE" '$0 != line' "$HOOK" > "$HOOK.asus-domain-query.tmp"
 if cmp -s "$HOOK.asus-domain-query.tmp" "$ROOT/services-start.before"; then
  if [ -f "$ROOT/hook-created" ]; then rm -f "$HOOK" "$HOOK.asus-domain-query.tmp"; else cp -p "$ROOT/services-start.before" "$HOOK"; rm -f "$HOOK.asus-domain-query.tmp"; fi
 else
  mv "$HOOK.asus-domain-query.tmp" "$HOOK"
 fi
fi
rm -rf "$ROOT"
rm -f /tmp/asus-domain-query/service.log
rmdir /tmp/asus-domain-query 2>/dev/null || true
echo 'Uninstalled; existing fancyss and firewall configuration preserved.'
