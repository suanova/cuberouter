# Copyright (C) 2023-2026 QuantumNous
# Copyright (C) 2026 CubeRouter
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as
# published by the Free Software Foundation, either version 3 of the
# License, or (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program. If not, see <https://www.gnu.org/licenses/>.
#
# For commercial licensing, please contact support@quantumnous.com
"""Local-only integration: real Go gateway + dispatcher, simulated GPU/Gemma.

Usage: python deploy/image-studio/smoke_api.py /absolute/path/to/cuberouter
Creates a disposable SQLite DB. Never connects to a deployed CubeRouter or GPU.
"""
import base64
import email
import email.policy
import hashlib
import json
import os
from pathlib import Path
import secrets
import socket
import sqlite3
import subprocess
import sys
import tempfile
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError
from urllib.request import Request, ProxyHandler, build_opener

from dispatcher import Server, channel_runner, channel_reconciler, channel_capacity
from queue_core import Dispatcher

PNG = base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=')
HTTP = build_opener(ProxyHandler({}))


def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]


def call(base, path, body=None, headers=None, method=None):
    req = Request(base + path, data=body if isinstance(body,bytes) else json.dumps(body).encode() if body is not None else None,
                  headers={'Content-Type': 'application/json', **(headers or {})}, method=method)
    try:
        r = HTTP.open(req, timeout=20)
    except HTTPError as exc:
        r = exc
    with r:
        return r.status, json.loads(r.read())


def wait_for(check, timeout=15):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        try:
            result = check()
            if result:
                return result
        except (OSError, ValueError):
            pass
        time.sleep(.1)
    raise AssertionError('local integration timed out')


class Provider(BaseHTTPRequestHandler):
    calls = []
    review = 'allow'
    active = 0
    peak = 0
    lock = threading.Lock()

    def log_message(self, *args): pass

    def reply(self, status, data):
        raw = json.dumps(data).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        self.reply(200, {'state': 'ready', 'gpu_count': 1, 'model': '2.1'})

    def do_POST(self):
        raw = self.rfile.read(int(self.headers.get('Content-Length', '0')))
        if self.path == '/v1/moderate':
            if Provider.review == 'unavailable':
                return self.reply(503, {})
            blocked = Provider.review == 'block'
            return self.reply(200, {'protocol': 'image-moderation-v1', 'model': 'google/shieldgemma-2-4b-it',
                'revision': 'a' * 40, 'weights_verified': True, 'image_sha256': hashlib.sha256(raw).hexdigest(),
                'decision': 'block' if blocked else 'allow', 'release_allowed': not blocked, 'threshold': .5,
                'scores': {'sexual': .8 if blocked else .01, 'violence': .02, 'dangerous': .03}})
        if self.headers.get('Content-Type','').startswith('multipart/form-data'):
            message=email.message_from_bytes(('Content-Type: '+self.headers['Content-Type']+'\r\n\r\n').encode()+raw,policy=email.policy.default)
            data={}
            for part in message.iter_parts():
                name=part.get_param('name',header='content-disposition')
                if name in ('image','image[]'): continue
                data[name]=part.get_payload(decode=True).decode()
            data['extra_fields']=json.loads(data['extra_fields'])
        else:
            data = json.loads(raw)
        if self.path == '/v1/chat/completions':
            return self.reply(200, {'id': 'local-chat', 'object': 'chat.completion', 'created': 1, 'model': 'gpt-local',
                'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': 'local-ok'}, 'finish_reason': 'stop'}],
                'usage': {'prompt_tokens': 2, 'completion_tokens': 2, 'total_tokens': 4}})
        with Provider.lock:
            Provider.active += 1
            Provider.peak = max(Provider.peak, Provider.active)
            Provider.calls.append(data)
        time.sleep(.5)
        with Provider.lock: Provider.active -= 1
        self.reply(200, {'created': int(time.time()), 'data': [{'b64_json': base64.b64encode(PNG).decode()}]})


def main():
    executable = str(Path(sys.argv[1]).resolve())
    provider = ThreadingHTTPServer(('127.0.0.1', 0), Provider)
    threading.Thread(target=provider.serve_forever, daemon=True).start()
    gpu = f'http://127.0.0.1:{provider.server_port}'
    gateway_port, dispatcher_port = port(), port()
    base = f'http://127.0.0.1:{gateway_port}'
    relay_key, queue_key, admin_pat = (secrets.token_hex(24) for _ in range(3))
    tokens = [secrets.token_hex(24) for _ in range(5)]
    process = server = queue = None
    with tempfile.TemporaryDirectory(prefix='image-api-smoke-') as tmp:
        root = Path(tmp)
        env = {**os.environ, 'SQL_DSN': 'local', 'LOG_SQL_DSN': '', 'SQLITE_PATH': str(root/'local.db'),
            'REDIS_CONN_STRING': '', 'MEMORY_CACHE_ENABLED': 'false', 'SESSION_SECRET': secrets.token_hex(32),
            'BIND_ADDRESS': '127.0.0.1', 'PORT': str(gateway_port), 'GIN_MODE': 'release',
            'IMAGE_STUDIO_CHANNEL_MODEL': 'qwen-image-2.1', 'IMAGE_STUDIO_CHANNEL_IDS': 'auto',
            'IMAGE_STUDIO_ALLOWED_GROUPS': 'image-studio', 'IMAGE_STUDIO_AUDIT_ENABLED': 'true',
            'IMAGE_STUDIO_RELAY_TOKEN': relay_key, 'IMAGE_STUDIO_DISPATCHER_TOKEN': queue_key,
            'IMAGE_STUDIO_DISPATCHER_URL': f'http://127.0.0.1:{dispatcher_port}',
            'IMAGE_STUDIO_MODERATION_ENABLED': 'true', 'IMAGE_STUDIO_MODERATION_URL': gpu+'/v1/moderate',
            'IMAGE_STUDIO_MODERATION_TOKEN': secrets.token_hex(24), 'BATCH_UPDATE_ENABLED': 'false'}
        log = open(root/'gateway.log', 'wb')
        def start():
            p = subprocess.Popen([executable, '--port', str(gateway_port)], cwd=tmp, env=env,
                                 stdout=log, stderr=log, creationflags=getattr(subprocess,'CREATE_NO_WINDOW',0))
            wait_for(lambda: call(base,'/api/setup')[0] == 200)
            return p
        def sql(query, values=()):
            with sqlite3.connect(root/'local.db', timeout=10) as db:
                db.row_factory = sqlite3.Row
                rows = db.execute(query, values).fetchall()
                return [dict(r) for r in rows]
        def insert(table, values):
            columns=','.join('"'+k+'"' for k in values)
            sql(f'INSERT INTO {table} ({columns}) VALUES ({",".join("?" for _ in values)})', tuple(values.values()))
        def auth(i=0): return {'Authorization': 'Bearer sk-'+tokens[i]}
        management = {'Authorization': 'Bearer '+admin_pat, 'New-Api-User': '1'}
        request = {'model': 'qwen-image-2.1', 'prompt': 'local safe landscape fixture', 'size': '512x512',
                   'n': 1, 'extra_fields': {'steps': 12, 'cfg': 2, 'seed': 123}}
        def generate(i=0, key=None, async_=False):
            return call(base, '/v1/images/generations', request,
                        {**auth(i), 'Idempotency-Key': key or secrets.token_hex(16), **({'Prefer':'respond-async'} if async_ else {})})
        try:
            process = start()
            password = 'Local1!'+secrets.token_hex(12)
            code, setup = call(base, '/api/setup', {'username':'localroot','password':password,'confirmPassword':password})
            assert code==200 and setup['success'], setup
            process.terminate(); process.wait(10); process=None
            sql('UPDATE users SET access_token=? WHERE id=1',(admin_pat,))
            for i, key in enumerate(tokens):
                insert('users',dict(id=201+i,username=f'local-image-{i}',password=sql('SELECT password FROM users WHERE id=1')[0]['password'] if '--preview' in sys.argv else '',role=1,status=1,
                                   quota=1000000,used_quota=0,request_count=0,setting='',group='image-studio' if i<4 else 'default'))
                insert('tokens',dict(id=301+i,user_id=201+i,key=key,status=1,name=f'local-key-{i}',
                                    expired_time=-1,remain_quota=100000,unlimited_quota=0,used_quota=0,
                                    model_limits_enabled=0,model_limits='',group=''))
            insert('channels',dict(id=11,type=1,key=secrets.token_hex(20),status=1,name='local-simulated',
                                  base_url=gpu,models='qwen-image-2.1,gpt-local',group='image-studio,default',
                                  channel_info=b'{}',used_quota=0,weight=1,priority=0))
            for group in ['image-studio','default']:
                for name in ['qwen-image-2.1','gpt-local']:
                    insert('abilities',dict(group=group,model=name,channel_id=11,enabled=1,priority=0,weight=1))
            for key,value in [('GroupRatio',{'default':1,'image-studio':1}),('ModelPrice',{'qwen-image-2.1':.001,'gpt-local':.001})]:
                sql('INSERT OR REPLACE INTO options (key,value) VALUES (?,?)',(key,json.dumps(value)))
            insert('models', dict(model_name='qwen-image-2.1', tags='text-to-image,image-to-image', status=1, sync_official=0))
            process = start()
            queue = Dispatcher([{'id':'local-slot','model':'2.1'}],channel_runner(base,relay_key),
                               reconcile=channel_reconciler(base,relay_key),capacity_reader=channel_capacity(base,relay_key))
            server = Server(('127.0.0.1',dispatcher_port),queue,queue_key,api_enabled=True)
            queue.start();threading.Thread(target=server.serve_forever,daemon=True).start()
            key=secrets.token_hex(16)
            status,result=generate(key=key)
            assert status==200 and result['data'][0]['b64_json']==base64.b64encode(PNG).decode(),(status,result)
            audit=sql('SELECT * FROM image_studio_audits ORDER BY id DESC LIMIT 1')[0]
            assert audit['source']=='api' and audit['moderation_state']=='external' and audit['token_id']==301
            assert audit['output_hash']==hashlib.sha256(PNG).hexdigest()
            assert json.loads(audit['parameters'])['steps']==12
            assert sql('SELECT used_quota FROM tokens WHERE id=301')[0]['used_quota']>0, 'API token was not charged'
            before=len(Provider.calls)
            assert generate(key=key)[0]==200
            assert len(Provider.calls)==before, 'duplicate inference'
            assert Provider.calls[0]['extra_fields']==request['extra_fields']
            print('PASS synchronous API, exact parameters, token billing, hashes, idempotent replay',flush=True)

            for review,expected in [('allow',200),('block',422),('unavailable',503)]:
                Provider.review=review
                code,result=call(base,'/api/image-api-policy',{'group':'image-studio','moderation':'required'},management,'PUT')
                assert code==200 and result['success'],(code,result)
                status,result=generate()
                assert status==expected,(review,status,result)
                if expected!=200: assert 'data' not in result
            print('PASS required review: approval, rejection and unavailable fail closed',flush=True)
            Provider.review='allow'
            with ThreadPoolExecutor(4) as pool:
                results=list(pool.map(lambda i:generate(i),range(4)))
            assert all(s==200 for s,_ in results),results
            assert Provider.peak==1,Provider.peak
            print('PASS four simultaneous accounts queued through one GPU slot',flush=True)

            boundary='local-image-fixture'
            multipart=b''
            for name,value in {'model':'qwen-image-2.1','prompt':'safe edit fixture','size':'512x512',
                               'extra_fields':json.dumps(request['extra_fields'])}.items():
                multipart+=f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode()
            multipart+=f'--{boundary}\r\nContent-Disposition: form-data; name="image[]"; filename="input.png"\r\nContent-Type: image/png\r\n\r\n'.encode()+PNG+f'\r\n--{boundary}--\r\n'.encode()
            code,result=call(base,'/v1/images/edits',multipart,{**auth(),'Content-Type':'multipart/form-data; boundary='+boundary})
            assert code==200,(code,result)
            audit=sql('SELECT * FROM image_studio_audits ORDER BY id DESC LIMIT 1')[0]
            assert json.loads(audit['input_hashes'])==[hashlib.sha256(PNG).hexdigest()]
            assert audit['model_prompt']=='safe edit fixture'
            print('PASS multipart API editing, reference hash and shared moderation',flush=True)

            assert generate(4)[0]==403,'non-image account accessed image service'
            assert call(base,'/api/image-studio-audit',headers=auth())[0] in (401,403)
            code,result=call(base,'/api/image-studio-audit?source=api',headers=management)
            assert code==200 and result['success'] and len(result['data'])>=8,(code,result)
            assert all(r['source']=='api' for r in result['data'])
            code,result=call(base,'/v1/chat/completions',{'model':'gpt-local','messages':[{'role':'user','content':'hello'}]},auth(4))
            assert code==200 and result['choices'][0]['message']['content']=='local-ok',(code,result)
            print('PASS image membership, root-only audit and ordinary LLM API regression',flush=True)

            status,result=generate(async_=True)
            assert status==202,(status,result)
            job=result['id']
            assert call(base,'/v1/images/jobs/'+job,headers=auth(1))[0]==404
            wait_for(lambda: call(base,'/v1/images/jobs/'+job,headers=auth())[0]==200)
            # Simulate lost dispatcher state: durable idempotency must prevent replay.
            before=len(Provider.calls)
            server.shutdown();server.server_close();queue.stop();server=queue=None
            queue=Dispatcher([{'id':'local-slot','model':'2.1'}],channel_runner(base,relay_key))
            server=Server(('127.0.0.1',dispatcher_port),queue,queue_key,api_enabled=True)
            queue.start();threading.Thread(target=server.serve_forever,daemon=True).start()
            assert generate(key=key)[0]!=200
            assert len(Provider.calls)==before
            print('PASS async polling, token isolation and no replay after dispatcher restart',flush=True)
            if '--preview' in sys.argv:
                # Disposable loopback credentials only, for manual UI verification.
                print(json.dumps({'local_preview':base,'username':'localroot','password':password,'stop_file':str(root/'stop')}),flush=True)
                end=time.monotonic()+900
                while time.monotonic()<end and not (root/'stop').exists(): time.sleep(1)
        except Exception:
            # Fixture-only logs; no real customer input or deployment credentials.
            log.flush()
            print((root/'gateway.log').read_text(encoding='utf-8',errors='replace')[-7000:],file=sys.stderr)
            raise
        finally:
            if server: server.shutdown();server.server_close()
            if queue: queue.stop()
            if process: process.terminate();process.wait(10)
            log.close();provider.shutdown();provider.server_close()


if __name__=='__main__': main()
