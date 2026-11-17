#!/bin/sh
# Owns only this application's process. Never restarts fancyss/httpd.
set -eu
ROOT=/jffs/asus-domain-query
RUN=/tmp/asus-domain-query
BIN=$ROOT/domain-query
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/koolshare/bin
export PATH
umask 077
mkdir -p "$RUN"
[ ! -L "$RUN" ] || exit 1
owned() {
 [ -f "$RUN/pid" ] || return 1
 PID=$(cat "$RUN/pid")
 case "$PID" in ''|*[!0-9]*) return 1;; esac
 [ "$(readlink "/proc/$PID/exe" 2>/dev/null)" = "$BIN" ] && kill -0 "$PID" 2>/dev/null
}
case "${1:-status}" in
 start)
  owned && { echo 'Already running'; exit 0; }
  mkdir "$RUN/start-lock" 2>/dev/null || { echo 'Start in progress'; exit 1; }
  trap 'rmdir "$RUN/start-lock" 2>/dev/null || true' EXIT
  LAN=$(nvram get lan_ipaddr)
  MASK=$(nvram get lan_netmask)
  IFACE=$(nvram get lan_ifname)
  [ "$IFACE" = br0 ] || { echo 'Expected LAN interface br0; refusing to bind elsewhere'; exit 1; }
  "$BIN" -lan "$LAN" -mask "$MASK" -interface "$IFACE" -check
  (trap '' HUP; exec "$BIN" -lan "$LAN" -mask "$MASK" -interface "$IFACE" </dev/null >"$RUN/service.log" 2>&1) &
  echo "$!" > "$RUN/pid"
  sleep 1
  owned || { cat "$RUN/service.log"; rm -f "$RUN/pid"; exit 1; }
  echo "Started: http://$LAN:18085/"
  ;;
 stop)
  if owned; then
   kill "$PID"
   count=0
   while owned && [ "$count" -lt 5 ]; do sleep 1; count=$((count + 1)); done
   owned && { echo 'Process has not stopped; refusing to remove files'; exit 1; }
  fi
  rm -f "$RUN/pid"
  echo 'Stopped'
  ;;
 status)
  if owned; then echo "Running PID $PID at http://$(nvram get lan_ipaddr):18085/"; else echo 'Not running'; exit 1; fi
  ;;
 *) echo 'Usage: service.sh start|stop|status'; exit 2;;
esac
