"""No real credentials, network requests or service changes."""
import hashlib,hmac,io,json,subprocess,tempfile,types,unittest

from pathlib import Path
from unittest.mock import patch

module=types.ModuleType('agent_repair')
source=Path(__file__).resolve().parents[1]/'internal/api/bootstrap/repair.py'
exec(compile(source.read_text(encoding='utf-8'),str(source),'exec'),module.__dict__)

class AgentRepairTests(unittest.TestCase):
    def test_proves_origin_without_sending_token(self):
        values={'HOST_ID':'a'*32,'AGENT_TOKEN':'test-identity-not-a-real-token'}
        def response(request,**kwargs):
            self.assertNotIn(values['AGENT_TOKEN'],str(request.headers)+request.data.decode())
            self.assertNotIn('Authorization',request.headers)
            self.assertTrue(request.get_header('User-agent').startswith('Starhub-Agent-Repair/'))
            body=json.loads(request.data)
            key=hashlib.sha256(values['AGENT_TOKEN'].encode()).hexdigest().encode()
            proof=hmac.new(key,('starhub-control-origin-v1:'+body['nonce']+':'+body['origin']).encode(),hashlib.sha256).hexdigest()
            return io.BytesIO(json.dumps({'proof':proof}).encode())
        with patch.object(module.urllib.request,'build_opener') as opener:
            opener.return_value.open.side_effect=response
            module.verify('https://control.example',values)
            module.verify('http://control.example:18080',values,True)
    def test_wrong_proof_and_plaintext_are_rejected(self):
        values={'HOST_ID':'a'*32,'AGENT_TOKEN':'fixture'}
        with patch.object(module.urllib.request,'build_opener') as opener:
            opener.return_value.open.return_value=io.BytesIO(b'{"proof":"fake"}')
            with self.assertRaisesRegex(ValueError,'does not prove'):module.verify('https://control.example',values)
        for origin in ('http://control.example','https://u:p@control.example','https://control.example/path'):
            with self.assertRaises(ValueError):module.verify(origin,values)
    def test_control_download_refuses_redirects(self):
        with self.assertRaisesRegex(ValueError,'Redirect refused'):
            module.NoRedirect().redirect_request(None,None,302,'',{},'https://other.example/agent.zip')

class ServiceUpgradeTests(unittest.TestCase):
    def setUp(self):
        folder=tempfile.TemporaryDirectory();self.addCleanup(folder.cleanup)
        self.root=Path(folder.name)
        self.legacy=self.root/'harbor-agent.service'
        self.current=self.root/'nodehub-agent.service'
        self.legacy.write_text('original unit contents')
        self.binary=self.root/'harbor-agent';self.binary.write_bytes(b'old')
        self.candidate=self.root/'candidate';self.candidate.write_bytes(b'new')
        self.active={self.legacy.name};self.enabled={self.legacy.name};self.failure=None
        for mock in (patch.object(module,'systemctl',side_effect=self.systemctl,create=True),patch.object(module.time,'sleep')):
            mock.start();self.addCleanup(mock.stop)

    def systemctl(self,*args,check=True,timeout=40):
        action,*rest=args
        if action=='daemon-reload':return ''
        name=rest[-1]
        if action=='is-enabled':return 'enabled' if name in self.enabled else 'disabled'
        if action=='is-active':return 'active' if name in self.active else 'inactive'
        if (action,name)==self.failure:raise subprocess.CalledProcessError(1,args)
        if action=='stop':self.active.discard(name)
        if action=='disable':self.enabled.discard(name)
        if action=='start':
            self.assertFalse(self.active,'both units would be running')
            self.active.add(name)
        if action=='enable':
            self.enabled.add(name)
            if '--now' in rest:self.active.add(name)
        return ''

    def test_migrates_unit_and_keeps_binary_path(self):
        module.activate_upgrade(self.binary,self.candidate,self.legacy,self.current)
        self.assertFalse(self.legacy.exists())
        self.assertEqual(self.current.read_text(),'original unit contents')
        self.assertEqual(self.binary.read_bytes(),b'new')
        self.assertEqual(self.active,{self.current.name})
        self.assertEqual(self.enabled,{self.current.name})

    def test_migration_failure_restores_legacy_unit_and_binary(self):
        for action in ('start','enable'):
            with self.subTest(action=action):
                self.failure=(action,self.current.name)
                self.candidate.write_bytes(b'new')
                with self.assertRaises(subprocess.CalledProcessError):
                    module.activate_upgrade(self.binary,self.candidate,self.legacy,self.current)
                self.assertFalse(self.current.exists())
                self.assertEqual(self.legacy.read_text(),'original unit contents')
                self.assertEqual(self.binary.read_bytes(),b'old')
                self.assertEqual(self.active,{self.legacy.name})
                self.assertEqual(self.enabled,{self.legacy.name})

    def test_current_unit_failure_does_not_rename_to_legacy(self):
        self.legacy.rename(self.current)
        self.active={self.current.name};self.enabled={self.current.name}
        original=self.systemctl
        def fail_once(*args,**kwargs):
            if args==('start',self.current.name) and self.binary.read_bytes()==b'new':
                raise subprocess.CalledProcessError(1,args)
            return original(*args,**kwargs)
        with patch.object(module,'systemctl',side_effect=fail_once),self.assertRaises(subprocess.CalledProcessError):
            module.activate_upgrade(self.binary,self.candidate,self.current,self.current)
        self.assertFalse(self.legacy.exists())
        self.assertEqual(self.binary.read_bytes(),b'old')
        self.assertEqual(self.active,{self.current.name})

    def test_failed_upgrade_restores_control_address_before_old_service_starts(self):
        env=self.root/'agent.env';env.write_bytes(b'old environment')
        self.failure=('start',self.current.name)
        def check_env(*args,**kwargs):
            if args==('start',self.legacy.name):self.assertEqual(env.read_bytes(),b'old environment')
            return self.systemctl(*args,**kwargs)
        with patch.object(module,'systemctl',side_effect=check_env),self.assertRaises(subprocess.CalledProcessError):
            module.activate_upgrade(self.binary,self.candidate,self.legacy,self.current,
                lambda:module.write_env(env,b'new environment'),lambda:module.write_env(env,b'old environment'))
        self.assertEqual(env.read_bytes(),b'old environment')

if __name__=='__main__':unittest.main()
