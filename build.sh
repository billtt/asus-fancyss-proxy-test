#!/bin/sh
set -eu
cd "$(dirname "$0")"
ROOT=$(pwd)
GO=${GO:-go}
mkdir -p work dist/asus-domain-query
export GOCACHE="$ROOT/work/go-cache" GOPATH="$ROOT/work/go-path" GOTOOLCHAIN=local
for script in install.sh uninstall.sh service.sh build.sh install-entry.sh remove-entry.sh entry/uninstall_asusdomainquery.sh; do /bin/sh -n "$script"; done
python3 tests/test_lifecycle.py
"$GO" test -race -count=1 -v ./src
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 "$GO" build -trimpath -ldflags='-s -w' -o dist/asus-domain-query/domain-query ./src
cp install.sh uninstall.sh service.sh install-entry.sh remove-entry.sh dist/asus-domain-query/
mkdir -p dist/asus-domain-query/entry
cp entry/* dist/asus-domain-query/entry/
cp README.md VERIFICATION.md dist/asus-domain-query/
cp "$("$GO" env GOROOT)/LICENSE" dist/asus-domain-query/GO-LICENSE
python3 - <<'PY'
from pathlib import Path
import hashlib,tarfile
p=Path('dist/asus-domain-query')
files=sorted(x for x in p.rglob("*") if x.is_file() and x.name!='SHA256SUMS')
(p/'SHA256SUMS').write_text(''.join(hashlib.sha256(x.read_bytes()).hexdigest()+'  '+x.relative_to(p).as_posix()+'\n' for x in files))
def public_metadata(info):
    info.uid=info.gid=0
    info.uname=info.gname=''
    info.mtime=0
    info.pax_headers={}
    return info
with tarfile.open('dist/asus-domain-query-aarch64.tar.gz','w:gz') as t:
    t.add(p,arcname=p.name,filter=public_metadata)
PY
