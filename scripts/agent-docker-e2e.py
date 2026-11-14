#!/usr/bin/env python3
"""Docker-isolated panel/agent/subscription/real-Xray smoke test.

Requires local images nodehub-xui-e2e:local and nodehub-agent:local.
Build the panel image with: docker build -t nodehub-xui-e2e:local .
No ports are published. The test removes only resources it creates.
"""
import base64
import json
import os
import secrets
import subprocess
import time
import urllib.parse
import uuid

prefix = 'nh-e2e-' + secrets.token_hex(5)
panel, agent, target = (prefix + '-' + s for s in ('panel', 'agent', 'target'))
network, target_net = prefix + '-net', prefix + '-target-net'
panel_vol, agent_vol = prefix + '-panel-vol', prefix + '-agent-vol'
csrf = ''
panel_image = os.environ.get('E2E_PANEL_IMAGE', 'nodehub-xui-e2e:local')
agent_image = os.environ.get('E2E_AGENT_IMAGE', 'nodehub-agent:local')
subnet_base = '11.' + str(secrets.randbelow(200) + 20) + '.' + str(secrets.randbelow(256))
target_ip = subnet_base + '.3'


def docker(*args, data=None, check=True):
    p = subprocess.run(['docker', *args], input=data, text=True, capture_output=True)
    if check and p.returncode:
        raise RuntimeError('Docker command failed: ' + ' '.join(args[:3]))
    return p


def request(path, body=None, post=False, base='http://127.0.0.1:2053'):
    args = ['exec', '-i', panel, 'curl', '-sS', '--max-time', '60', '-b', '/tmp/e2e-cookie', '-c', '/tmp/e2e-cookie', '-w', '\n%{http_code}']
    if post:
        args += ['-X', 'POST', '-H', 'X-CSRF-Token: ' + csrf]
    if body is not None:
        args += ['-H', 'Content-Type: application/json', '--data-binary', '@-']
    p = docker(*args, base + path, data=json.dumps(body) if body is not None else '')
    content, status = p.stdout.rsplit('\n', 1)
    if status != '200':
        raise RuntimeError('HTTP ' + status + ' on ' + path.split('?')[0])
    return content


def api(path, body=None, post=False):
    value = json.loads(request(path, body, post))
    if not value.get('success'):
        raise RuntimeError('API failed ' + path + ': ' + str(value.get('msg', '')))
    return value.get('obj')


def proxy():
    return docker('exec', agent, 'curl', '-sS', '--noproxy', '', '--socks5-hostname', '127.0.0.1:10808', '--max-time', '4', '-w', '\n%{http_code}', 'http://' + target_ip + ':18080/', check=False)


def await_clients(node_id, count):
    for i in range(25):
        time.sleep(1)
        node = api('/panel/api/nodes/get/' + str(node_id))
        config = docker('exec', agent, 'cat', '/var/lib/nodehub-agent/config.json', check=False)
        if config.returncode == 0:
            data = json.loads(config.stdout)
            if not node['configDirty'] and len(data['inbounds'][0]['settings']['clients']) == count:
                return i + 1
    raise RuntimeError('automatic reconciliation did not converge')


try:
    docker('network', 'create', '--internal', network)
    # Xray freedom blackholes RFC1918 addresses. This globally classified range
    # is confined to an internal Docker network, without an external gateway.
    docker('network', 'create', '--internal', '--subnet', subnet_base + '.0/24', target_net)
    docker('volume', 'create', panel_vol)
    docker('volume', 'create', agent_vol)
    docker('run', '--rm', '--network', 'none', '--user', '0', '--entrypoint', '/bin/sh', '-v', agent_vol + ':/state', agent_image, '-c', 'umask 077; head -c 32 /dev/urandom | od -An -tx1 | tr -d " \\n" > /state/token; chown -R 10001:10001 /state; chmod 700 /state')
    docker('run', '-d', '--name', agent, '--network', network, '--network-alias', 'agent', '--read-only', '--tmpfs', '/tmp', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges', '-v', agent_vol + ':/var/lib/nodehub-agent', agent_image, '-listen', '0.0.0.0:9443', '-allow-http', '-token-file', '/var/lib/nodehub-agent/token', '-identity-file', '/var/lib/nodehub-agent/identity', '-config-file', '/var/lib/nodehub-agent/config.json', '-xray', '/usr/local/bin/xray')
    docker('run', '-d', '--name', panel, '--network', network, '--network-alias', 'panel', '--security-opt', 'no-new-privileges', '-v', panel_vol + ':/etc/x-ui', '-e', 'XUI_DB_TYPE=sqlite', '-e', 'XUI_DB_FOLDER=/etc/x-ui', '-e', 'XUI_PORT=2053', '-e', 'XUI_INIT_WEB_BASE_PATH=/', '-e', 'XUI_ENABLE_FAIL2BAN=false', panel_image)
    docker('run', '-d', '--name', target, '--network', network, '--read-only', '--tmpfs', '/tmp', '--security-opt', 'no-new-privileges', '--cap-drop', 'ALL', '--entrypoint', '/bin/sh', agent_image, '-c', 'while true; do printf "HTTP/1.1 200 OK\\r\\nContent-Length: 18\\r\\nConnection: close\\r\\n\\r\\nisolated-target-ok" | nc -l -p 18080; done')
    docker('network', 'connect', '--ip', subnet_base + '.2', target_net, agent)
    docker('network', 'connect', '--ip', target_ip, target_net, target)
    for _ in range(30):
        try:
            csrf = api('/csrf-token')
            break
        except (RuntimeError, ValueError):
            time.sleep(1)
    else:
        raise RuntimeError('panel did not start')
    api('/login', {'username': 'admin', 'password': 'admin'}, True)
    csrf = api('/csrf-token')
    token = docker('exec', agent, 'cat', '/var/lib/nodehub-agent/token').stdout.strip()
    node = api('/panel/api/nodes/add', {'name': 'e2e', 'nodeType': 'agent', 'scheme': 'http', 'address': 'agent', 'port': 9443, 'basePath': '/', 'apiToken': token, 'enable': True, 'allowPrivateAddress': True, 'tlsVerifyMode': 'verify', 'inboundSyncMode': 'all'}, True)
    node_id = node['id']
    inbound = api('/panel/api/inbounds/add', {'remark': 'e2e', 'enable': True, 'port': 24443, 'protocol': 'vless', 'nodeId': node_id, 'settings': json.dumps({'clients': [], 'decryption': 'none'}), 'streamSettings': json.dumps({'network': 'tcp', 'security': 'none'}), 'sniffing': json.dumps({'enabled': False})}, True)
    client_id = str(uuid.uuid4())
    api('/panel/api/clients/add', {'client': {'id': client_id, 'email': 'e2e@example.com', 'enable': True}, 'inboundIds': [inbound['id']]}, True)
    print('PASS automatic apply and dirty clear in', await_clients(node_id, 1), 'seconds')
    sub_id = api('/panel/api/clients/get/e2e@example.com')['client']['subId']
    settings = api('/panel/api/setting/all', post=True)
    raw = request(settings['subPath'] + sub_id, base='http://127.0.0.1:2096')
    link = urllib.parse.urlparse(base64.b64decode(raw).decode().strip())
    assert link.scheme == 'vless' and link.hostname == 'agent' and link.port == 24443 and link.username == client_id
    print('PASS generated subscription matches agent and client')
    config = {'log': {'loglevel': 'warning'}, 'inbounds': [{'listen': '127.0.0.1', 'port': 10808, 'protocol': 'socks', 'settings': {'auth': 'noauth'}}], 'outbounds': [{'protocol': 'vless', 'settings': {'vnext': [{'address': '127.0.0.1', 'port': link.port, 'users': [{'id': link.username, 'encryption': 'none'}]}]}, 'streamSettings': {'network': 'tcp', 'security': 'none'}}]}
    encoded = base64.b64encode(json.dumps(config).encode()).decode()
    docker('exec', agent, 'sh', '-c', 'printf %s ' + encoded + ' | base64 -d >/tmp/client.json')
    docker('exec', '-d', agent, '/usr/local/bin/xray', 'run', '-c', '/tmp/client.json')
    time.sleep(1)
    result = proxy()
    assert result.returncode == 0 and result.stdout == 'isolated-target-ok\n200'
    print('PASS real Xray VLESS proxy HTTP 200')
    old_config = json.loads(docker('exec', agent, 'cat', '/var/lib/nodehub-agent/config.json').stdout)
    fence_path = docker('exec', agent, 'sh', '-c', 'printf %s /var/lib/nodehub-agent/config.json.version').stdout.strip()
    fence_result = docker('exec', agent, 'cat', fence_path, check=False)
    if fence_result.returncode:
        listing = docker('exec', agent, 'ls', '-la', '/var/lib/nodehub-agent', check=False).stdout
        raise RuntimeError('agent version state missing; state directory: ' + listing)
    old_fence = json.loads(docker('exec', agent, 'cat', fence_path).stdout)
    docker('network', 'disconnect', network, agent)
    api('/panel/api/nodes/probe/' + str(node_id), {}, True)
    response = json.loads(request('/panel/api/nodes/setEnable/' + str(node_id), {'enable': False}, True))
    assert not response['success'] and response['obj']['status'] in ('unknown', 'failed')
    node = api('/panel/api/nodes/get/' + str(node_id))
    assert not node['enable'] and node['configDirty']
    offline_proxy = proxy()
    assert offline_proxy.returncode == 0 and offline_proxy.stdout == 'isolated-target-ok\n200', repr((offline_proxy.returncode, offline_proxy.stdout, offline_proxy.stderr))
    print('PASS offline disable remains dirty and old proxy remains live until acknowledged')
    docker('network', 'connect', '--alias', 'agent', network, agent)
    for elapsed in range(1, 46):
        time.sleep(1)
        node = api('/panel/api/nodes/get/' + str(node_id))
        if not node['configDirty']:
            break
    else:
        raise RuntimeError('offline disable retry did not converge')
    assert proxy().returncode != 0
    fence = json.loads(docker('exec', agent, 'cat', '/var/lib/nodehub-agent/config.json.version').stdout)
    assert fence['operation'] == 'disable' and fence['applied'] and fence['config_version'] > old_fence['config_version']
    print('PASS offline recovery retried node disable; proxy rejected in', elapsed, 'seconds')
    replay = docker('exec', '-e', 'AGENT_TOKEN=' + token, '-i', agent, 'sh', '-c', 'curl -sS -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $AGENT_TOKEN" -H "Content-Type: application/json" --data-binary @- http://127.0.0.1:9443/v1/config', data=json.dumps({'config_version': old_fence['config_version'], 'config': old_config}))
    assert replay.stdout == '409' and proxy().returncode != 0
    print('PASS stale config replay rejected HTTP 409; proxy remains disabled')
    docker('restart', agent)
    time.sleep(2)
    status = docker('exec', '-e', 'AGENT_TOKEN=' + token, agent, 'sh', '-c', 'curl -fsS -H "Authorization: Bearer $AGENT_TOKEN" http://127.0.0.1:9443/v1/status')
    assert json.loads(status.stdout)['xray_status'] == 'stopped'
    replay = docker('exec', '-e', 'AGENT_TOKEN=' + token, '-i', agent, 'sh', '-c', 'curl -sS -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $AGENT_TOKEN" -H "Content-Type: application/json" --data-binary @- http://127.0.0.1:9443/v1/config', data=json.dumps({'config_version': old_fence['config_version'], 'config': old_config}))
    assert replay.stdout == '409'
    docker('exec', agent, 'sh', '-c', 'printf %s ' + encoded + ' | base64 -d >/tmp/client.json')
    docker('exec', '-d', agent, '/usr/local/bin/xray', 'run', '-c', '/tmp/client.json')
    time.sleep(1)
    assert proxy().returncode != 0
    print('PASS disabled startup and version fence persist across agent restart')
    assert api('/panel/api/nodes/setEnable/' + str(node_id), {'enable': True}, True)['status'] == 'applied'
    assert proxy().stdout == 'isolated-target-ok\n200'
    print('PASS node re-enable restored real Xray proxy HTTP 200')
    api('/panel/api/clients/update/e2e@example.com', {'enable': False}, True)
    elapsed = await_clients(node_id, 0)
    assert proxy().returncode != 0
    print('PASS disable auto-applied and proxy rejected in', elapsed, 'seconds')
    api('/panel/api/clients/update/e2e@example.com', {'enable': True}, True)
    elapsed = await_clients(node_id, 1)
    result = proxy()
    assert result.returncode == 0 and result.stdout == 'isolated-target-ok\n200'
    print('PASS re-enable auto-applied and proxy restored in', elapsed, 'seconds')
finally:
    for name in (target, panel, agent):
        docker('rm', '-f', '-v', name, check=False)
    for name in (network, target_net):
        docker('network', 'rm', name, check=False)
    for name in (panel_vol, agent_vol):
        docker('volume', 'rm', name, check=False)