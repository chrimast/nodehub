"""Repair an existing Linux Agent control origin without changing its identity."""
import hashlib,hmac,json,os,platform,re,secrets,shutil,stat,subprocess,sys,tempfile,time,urllib.parse,urllib.request,zipfile
from pathlib import Path

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,*args):raise ValueError('Redirect refused')

VERSION='__PANEL_VERSION__'
AGENT_VERSION='__AGENT_VERSION__'

UNIT_NAMES=('nodehub-agent.service','harbor-agent.service')

def systemctl(*args,check=True,timeout=40):
    return subprocess.run(['systemctl',*args],check=check,timeout=timeout,text=True,stdout=subprocess.PIPE).stdout.strip()

def agent_unit():
    names=[]
    for name in UNIT_NAMES:
        state=systemctl('show','--property=LoadState','--value',name,check=False)
        path=Path('/etc/systemd/system')/name
        if state=='loaded':names.append(name)
        elif state!='not-found' or path.exists() or path.is_symlink():raise ValueError('Unexpected Agent unit state: '+name)
    if len(names)!=1:raise ValueError('Expected exactly one Agent service; inspect both unit names before retrying')
    return names[0]

def activate_upgrade(binary,candidate,old,new,apply_env=lambda:None,restore_env=lambda:None):
    migrate=old!=new
    if migrate and (new.exists() or new.is_symlink() or systemctl('show','--property=DropInPaths','--value',old.name)):
        raise ValueError('Agent unit conflict or custom drop-ins; migrate the unit manually before upgrading')
    enabled=systemctl('is-enabled',old.name,check=False)
    active=systemctl('is-active',old.name,check=False)
    if enabled not in ('enabled','disabled') or active not in ('active','inactive','failed'):
        raise ValueError('Unsupported Agent unit state; inspect systemctl status before retrying')
    backup=binary.with_name(binary.name+'.previous')
    previous=candidate.with_name('previous')
    shutil.copy2(binary,previous)
    os.replace(previous,backup)
    moved=replaced=enabling=env_changed=False
    try:
        systemctl('stop',old.name)
        if migrate:
            if enabled=='enabled':systemctl('disable',old.name)
            os.rename(old,new);moved=True
            systemctl('daemon-reload')
        os.replace(candidate,binary);replaced=True
        apply_env();env_changed=True
        systemctl('start',new.name)
        time.sleep(2)
        if systemctl('is-active',new.name,check=False)!='active':raise ValueError('Updated Agent service did not stay active')
        if migrate and enabled=='enabled':
            enabling=True
            systemctl('enable',new.name)
        if active!='active':systemctl('stop',new.name)
    except BaseException as failure:
        try:
            if replaced:
                systemctl('stop',new.name)
                shutil.copy2(backup,previous)
                os.replace(previous,binary)
            if moved:
                if enabling:systemctl('disable',new.name)
                os.rename(new,old)
                systemctl('daemon-reload')
            if migrate and enabled=='enabled':systemctl('enable',old.name)
            if env_changed:restore_env()
            if active=='active':systemctl('start',old.name)
        except Exception as rollback_error:
            raise RuntimeError(f'Agent upgrade failed ({failure}); rollback failed ({rollback_error}). Previous binary: {backup}') from rollback_error
        raise

def upgrade_agent(origin, env_path, service, apply_env, restore_env):
    arch={'x86_64':'amd64','aarch64':'arm64'}.get(platform.machine())
    if not arch:raise ValueError('Unsupported Linux architecture')
    binary=Path('/opt/harbor-agent/harbor-agent') if env_path==Path('/etc/harbor-agent/agent.env') else Path('/opt/harbor/bin/harbor-agent')
    old=Path('/etc/systemd/system')/service
    unit_stat=old.lstat()
    if (not stat.S_ISREG(unit_stat.st_mode) or unit_stat.st_uid!=0 or unit_stat.st_mode&0o022
        or systemctl('show','--property=FragmentPath','--value',service)!=str(old)
        or not {f'ExecStart={binary}',f'EnvironmentFile={env_path}'}<=set(old.read_text().splitlines())):
        raise ValueError('Root-owned standard Agent service required; use the panel updater for custom layouts')
    backup=binary.with_name('harbor-agent.previous')
    for path in (binary,binary.parent,backup if backup.exists() or backup.is_symlink() else binary.parent):
        st=path.lstat()
        if path.is_symlink() or st.st_uid!=0 or st.st_mode&0o022:raise ValueError('Root-owned standard Agent installation required')
    url=origin+'/api/v1/agent/releases/linux-'+arch
    pinned=next((arg.split('=',1)[1] for arg in sys.argv[2:] if arg.startswith('--package-'+arch+'=')),None)
    if origin.startswith('http://') and not re.fullmatch('[a-f0-9]{64}',pinned or ''):raise ValueError('HTTP upgrade requires a pinned package checksum')
    opener=urllib.request.build_opener(urllib.request.ProxyHandler({}),NoRedirect())
    with opener.open(url,timeout=30) as response:asset=json.loads(response.read(2048))
    if not isinstance(asset.get('size'),int) or not 0<asset['size']<=160<<20 or not re.fullmatch('[a-f0-9]{64}',asset.get('sha256','')):raise ValueError('Agent archive checksum missing')
    if pinned and not hmac.compare_digest(pinned,asset['sha256']):raise ValueError('Agent package changed; generate a new command')
    if shutil.disk_usage(binary.parent).free<asset['size']*2+(64<<20):raise ValueError('Not enough space to preserve the previous Agent')
    with tempfile.TemporaryDirectory(prefix='.upgrade-',dir=binary.parent) as folder:
        package=Path(folder)/'agents.zip';sha=hashlib.sha256();size=0
        with opener.open(url+'?download=1',timeout=120) as response,package.open('xb') as out:
            while chunk:=response.read(1<<20):
                size+=len(chunk)
                if size>asset['size']:raise ValueError('Oversized Agent archive')
                sha.update(chunk);out.write(chunk)
        if size!=asset['size'] or sha.hexdigest()!=(pinned or asset['sha256']):raise ValueError('Agent archive checksum mismatch')
        candidate=Path(folder)/'harbor-agent'
        network='--enable-port-hopping' in sys.argv or Path('/usr/local/libexec/starhub-agent-network.py').exists()
        helper=None
        staged_cleanup=None
        cleanup=Path('/usr/local/libexec/harbor-agent-cleanup')
        refresh_cleanup=network or cleanup.exists() or cleanup.is_symlink() or Path('/etc/harbor-agent-cleanup.json').exists()
        with zipfile.ZipFile(package) as z:
            if network:
                info=z.getinfo('agent-network.py')
                if info.file_size>65536:raise ValueError('Oversized network helper')
                helper=Path(folder)/'agent-network.py';helper.write_bytes(z.read(info))
            if refresh_cleanup:
                if cleanup.is_symlink() or cleanup.exists() and (not cleanup.is_file() or cleanup.stat().st_uid!=0 or cleanup.stat().st_mode&0o022):raise ValueError('Untrusted cleanup helper')
                cleanup_info=z.getinfo('harbor-agent-cleanup')
                if cleanup_info.file_size>64<<20:raise ValueError('Oversized cleanup helper')
                staged_cleanup=Path(folder)/'harbor-agent-cleanup';staged_cleanup.write_bytes(z.read(cleanup_info));staged_cleanup.chmod(0o755)

            items=[f for f in z.infolist() if f.filename=='harbor-agent']
            if len(items)!=1 or items[0].file_size>64<<20:raise ValueError('Invalid Agent executable')
            with z.open(items[0]) as source,candidate.open('xb') as out:shutil.copyfileobj(source,out)
        candidate.chmod(0o755)
        version=subprocess.run([str(candidate),'--version'],check=True,capture_output=True,text=True,timeout=15).stdout.strip()
        if version!=AGENT_VERSION:raise ValueError('Downloaded Agent version mismatch')
        # The refreshed janitor supports either unit, including after rollback.
        if refresh_cleanup and staged_cleanup is not None:
            cleanup.parent.mkdir(parents=True,exist_ok=True)
            with tempfile.NamedTemporaryFile(dir=cleanup.parent,delete=False) as out:
                replacement=Path(out.name)
            try:
                shutil.copy2(staged_cleanup,replacement)
                os.replace(replacement,cleanup)
            finally:replacement.unlink(missing_ok=True)
        if network and helper is not None:subprocess.run([sys.executable,str(helper),'--install','--env',str(env_path)],check=True,timeout=60)
        activate_upgrade(binary,candidate,old,old.with_name(UNIT_NAMES[0]),apply_env,restore_env)
    print('Agent upgraded to '+AGENT_VERSION+'. Existing cores, credentials and traffic ledger were preserved. Confirm the version and heartbeat in the panel.')

def verify(origin,values,allow_http=False):
    u=urllib.parse.urlsplit(origin)
    if (u.scheme!='https' and not (allow_http and u.scheme=='http')) or not u.hostname or u.username or u.password or u.path or u.query or u.fragment:raise ValueError('Expected an HTTPS origin, or an explicitly allowed HTTP origin')
    nonce=secrets.token_hex(32)
    body=json.dumps({'host_id':values['HOST_ID'],'nonce':nonce,'origin':origin,'allow_insecure_http':allow_http}).encode()
    req=urllib.request.Request(origin+'/api/v1/agent/verify-origin',data=body,headers={'Content-Type':'application/json','User-Agent':'Starhub-Agent-Repair/'+VERSION})
    with urllib.request.build_opener(urllib.request.ProxyHandler({}),NoRedirect()).open(req,timeout=15) as response:
        proof=json.loads(response.read(2048))['proof']
    key=hashlib.sha256(values['AGENT_TOKEN'].encode()).hexdigest().encode()
    expected=hmac.new(key,('starhub-control-origin-v1:'+nonce+':'+origin).encode(),hashlib.sha256).hexdigest()
    if not hmac.compare_digest(proof,expected):raise ValueError('New endpoint does not prove this Agent identity; nothing changed')

def write_env(path, data):
    temporary=path.with_name('agent.env.repair-'+secrets.token_hex(8))
    try:
        with temporary.open('xb') as out:out.write(data);out.flush();os.fsync(out.fileno())
        temporary.chmod(0o600);os.replace(temporary,path)
    finally:temporary.unlink(missing_ok=True)

def main():
    if os.geteuid()!=0:raise ValueError('Run as root on the existing Agent host')
    origin=sys.argv[1].rstrip('/')
    path=Path('/etc/harbor-agent/agent.env')
    if not path.exists() and Path('/etc/harbor/agent.env').exists():path=Path('/etc/harbor/agent.env')
    st=path.lstat()
    if not stat.S_ISREG(st.st_mode) or st.st_uid!=0 or st.st_mode&0o077:raise ValueError('Private root-owned Agent environment required')
    previous=path.read_bytes()
    lines=previous.decode().splitlines();values=dict(line.split('=',1) for line in lines if '=' in line and not line.startswith('#'))
    if not re.fullmatch('[a-f0-9]{32}',values.get('HOST_ID','')) or len(values.get('AGENT_TOKEN',''))<32:raise ValueError('Existing Agent identity missing')
    if sum(line.startswith('CONTROL_URL=') for line in lines)!=1:raise ValueError('Expected exactly one CONTROL_URL')
    allow_http=origin.startswith('http://') and '--allow-insecure-http' in sys.argv[2:]
    verify(origin,values,allow_http)
    service=agent_unit()
    updated='\n'.join(['CONTROL_URL='+origin,'ALLOW_INSECURE_HTTP='+('1' if allow_http else '0'),*(line for line in lines if not line.startswith(('CONTROL_URL=','ALLOW_INSECURE_HTTP=')))])+'\n'
    os.umask(0o077)
    apply_env=lambda:write_env(path,updated.encode())
    restore_env=lambda:write_env(path,previous)
    if '--upgrade-agent' in sys.argv[2:]:upgrade_agent(origin,path,service,apply_env,restore_env)
    else:
        apply_env()
        try:systemctl('restart',service)
        except Exception:
            restore_env()
            systemctl('restart',service)
            raise
    print('Control address repaired. Agent identity, proxy configuration and traffic ledger were preserved. Confirm the new heartbeat in the panel.')

if __name__=='__main__':
    try:main()
    except Exception as error:print('Repair stopped: '+str(error),file=sys.stderr);sys.exit(1)
