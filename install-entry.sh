#!/bin/sh
set -eu
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/koolshare/bin
export PATH
ROOT=/jffs/asus-domain-query
HERE=$(CDPATH= cd "$(dirname "$0")" && pwd)
[ -x "$ROOT/domain-query" ] || { echo 'Install the query tool first'; exit 1; }
[ ! -L "$ROOT" ] || exit 1
[ ! -f "$ROOT/entry-installed" ] || { echo 'Shortcut already installed'; exit 0; }
for f in /koolshare/webs/Module_asusdomainquery.asp /koolshare/res/icon-asusdomainquery.png /koolshare/scripts/uninstall_asusdomainquery.sh; do
 [ ! -e "$f" ] && [ ! -L "$f" ] || { echo "Refusing to overwrite $f"; exit 1; }
done
[ -z "$(dbus list softcenter_module_asusdomainquery_)" ] || { echo 'Shortcut metadata already exists'; exit 1; }
for f in Module_asusdomainquery.asp icon-asusdomainquery.png uninstall_asusdomainquery.sh; do [ -f "$HERE/entry/$f" ] || exit 1; done
[ -f "$HERE/remove-entry.sh" ] || exit 1
cp "$HERE/remove-entry.sh" "$ROOT/remove-entry.sh"
chmod 700 "$ROOT/remove-entry.sh"
umask 022
# Ownership marker lets cleanup remove only this new entry on an interrupted install.
touch "$ROOT/entry-installed"
DONE=0
trap '[ "$DONE" = 1 ] || "$ROOT/remove-entry.sh"' EXIT
cp "$HERE/entry/Module_asusdomainquery.asp" /koolshare/webs/
cp "$HERE/entry/icon-asusdomainquery.png" /koolshare/res/
cp "$HERE/entry/uninstall_asusdomainquery.sh" /koolshare/scripts/
chmod 644 /koolshare/webs/Module_asusdomainquery.asp /koolshare/res/icon-asusdomainquery.png
chmod 755 /koolshare/scripts/uninstall_asusdomainquery.sh
dbus set softcenter_module_asusdomainquery_name=asusdomainquery
dbus set softcenter_module_asusdomainquery_title='域名分流查询'
dbus set softcenter_module_asusdomainquery_description='LAN 只读查询入口；移除此入口不影响查询服务'
dbus set softcenter_module_asusdomainquery_version=1.0.0
dbus set 'softcenter_module_asusdomainquery_target=target=_blank'
# Publish last: the UI ignores entries without install metadata.
dbus set softcenter_module_asusdomainquery_install=4
DONE=1
echo 'Shortcut installed: 软件中心 → 已安装 → 域名分流查询'
