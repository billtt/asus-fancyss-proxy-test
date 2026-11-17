"""Exercise installation file operations in an isolated tree, never /jffs.
Only paths/platform probes and the process launcher are replaced for macOS.
"""
from pathlib import Path
import hashlib, shutil, subprocess, tempfile, unittest
PROJECT=Path(__file__).resolve().parents[1]
class Lifecycle(unittest.TestCase):
 def setUp(self):
  (PROJECT/'work').mkdir(exist_ok=True)
  self.root=Path(tempfile.mkdtemp(prefix='lifecycle-',dir=PROJECT/'work'))
  self.jffs=self.root/'jffs';(self.jffs/'scripts').mkdir(parents=True)
  self.hook=self.jffs/'scripts/services-start';self.original=b'#!/bin/sh\n/koolshare/bin/ks-services-start.sh\n'
  self.hook.write_bytes(self.original);self.hook.chmod(0o751)
  self.package=self.root/'package';self.package.mkdir();self.bin=self.root/'bin';self.bin.mkdir()
  uid=self.root/'status';uid.write_text('Uid:\t0\t0\t0\t0\n')
  for name in ['install.sh','uninstall.sh']:
   text=(PROJECT/name).read_text().replace('/jffs',str(self.jffs)).replace('/tmp/asus-domain-query',str(self.root/'runtime')).replace('/proc/self/status',str(uid))
   text=text.replace('PATH=/bin:/usr/bin:/sbin:/usr/sbin:/koolshare/bin','PATH='+str(self.bin)+':/usr/bin:/bin:/usr/sbin:/sbin')
   (self.package/name).write_text(text)
  self.script(self.bin/'nvram','case "$2" in jffs2_scripts) echo 1;; lan_ifname) echo br0;; lan_ipaddr) echo 10.23.45.1;; lan_netmask) echo 255.255.255.0;; esac')
  self.script(self.bin/'uname','echo aarch64')
  self.script(self.bin/'df',"printf 'Filesystem 1K-blocks Used Available Use%% Mounted\nmock 99999 1 99998 1%% /jffs\n'")
  checksum=shutil.which('sha256sum')
  if checksum:
   self.script(self.bin/'sha256sum',f'exec "{checksum}" "$@"')
  else:
   checksum=shutil.which('shasum')
   if not checksum:self.fail('sha256sum or shasum is required')
   self.script(self.bin/'sha256sum',f'exec "{checksum}" -a 256 "$@"')
  self.script(self.package/'domain-query','exit 0')
  self.script(self.package/'service.sh','exit 0')
  self.checksums()
 def script(self,path,body):path.write_text('#!/bin/sh\n'+body+'\n');path.chmod(0o700)
 def checksums(self):
  (self.package/'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in sorted(self.package.iterdir()) if p.name!='SHA256SUMS'))
 def run_script(self,name,success=True):
  r=subprocess.run(['/bin/sh',str(self.package/name)],capture_output=True,text=True)
  self.assertEqual(r.returncode==0,success,r.stdout+r.stderr);return r
 def tearDown(self):shutil.rmtree(self.root)
 def test_install_uninstall_exact_restore(self):
  self.run_script('install.sh');self.assertIn(b'asus-domain-query',self.hook.read_bytes());self.run_script('uninstall.sh');self.assertEqual(self.hook.read_bytes(),self.original);self.assertEqual(self.hook.stat().st_mode&0o777,0o751);self.assertFalse((self.jffs/'asus-domain-query').exists())
 def test_preserve_later_hook_changes(self):
  self.run_script('install.sh');self.hook.write_bytes(self.hook.read_bytes()+b'# later user edit\n');self.run_script('uninstall.sh');self.assertEqual(self.hook.read_bytes(),self.original+b'# later user edit\n')
 def test_reinstall_refused(self):
  self.run_script('install.sh');before=self.hook.read_bytes();self.run_script('install.sh',False);self.assertEqual(before,self.hook.read_bytes())
 def test_failed_start_rolls_back(self):
  self.script(self.package/'service.sh','[ "$1" != start ]');self.checksums();self.run_script('install.sh',False);self.assertEqual(self.hook.read_bytes(),self.original);self.assertFalse((self.jffs/'asus-domain-query').exists())
 def test_bad_checksum(self):
  (self.package/'domain-query').write_text('corrupt');self.run_script('install.sh',False);self.assertEqual(self.hook.read_bytes(),self.original)
 def test_created_hook_removed(self):
  self.hook.unlink();self.run_script('install.sh');self.run_script('uninstall.sh');self.assertFalse(self.hook.exists())
 def test_existing_temp_preserved(self):
  temp=Path(str(self.hook)+".asus-domain-query.tmp");temp.write_text("other owner")
  self.run_script("install.sh",False);self.assertEqual(temp.read_text(),"other owner")
 def test_symlink_rejected(self):
  self.hook.unlink();target=self.root/'important';target.write_bytes(self.original);self.hook.symlink_to(target);self.run_script('install.sh',False);self.assertEqual(target.read_bytes(),self.original)
if __name__=='__main__':unittest.main(verbosity=2)
