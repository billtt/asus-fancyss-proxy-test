#!/bin/sh
# Remove only the software-center shortcut, never the running query tool.
set -eu
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/koolshare/bin
export PATH
ROOT=/jffs/asus-domain-query
[ -f "$ROOT/entry-installed" ] || { echo 'No owned software-center shortcut'; exit 0; }
for field in install version name title description target; do
 dbus remove "softcenter_module_asusdomainquery_$field"
done
rm -f /koolshare/webs/Module_asusdomainquery.asp /koolshare/res/icon-asusdomainquery.png /koolshare/scripts/uninstall_asusdomainquery.sh
rm -f "$ROOT/entry-installed"
echo 'Software-center shortcut removed; query service is unchanged.'
