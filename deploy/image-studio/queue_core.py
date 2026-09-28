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

"""One RAM-only dispatcher; each configured channel has exactly one execution slot."""
import hashlib, json, secrets, threading, time
from collections import deque

class Rejected(Exception):
    def __init__(self, status, code): self.status, self.code = status, code

class BusyBeforeAccept(Exception): pass
class UncertainExecution(Exception): pass

class Dispatcher:
    def __init__(self, channels, runner, ready=lambda channel: True, *, queue_limit=10,
                 byte_limit=256*1024*1024, result_ttl=600, queue_ttl=900, metadata_ttl=600,
                 max_jobs=1000, clock=time.monotonic):
        if len({c['id'] for c in channels}) != len(channels): raise ValueError('Duplicate channel IDs')
        if len({c.get('url',c['id']) for c in channels}) != len(channels): raise ValueError('Duplicate worker endpoints')
        self.channels = [dict(c, state='idle', job=None) for c in channels]
        self.runner, self.ready, self.clock = runner, ready, clock
        self.queue_limit, self.byte_limit = queue_limit, byte_limit
        self.result_ttl, self.queue_ttl, self.metadata_ttl = result_ttl, queue_ttl, metadata_ttl
        self.max_jobs = max_jobs
        self.queues = {m:deque() for m in {c['model'] for c in channels}}
        self.jobs, self.keys = {}, {}
        self.cv = threading.Condition(threading.RLock())
        self.stopping = False
        self.threads = []

    def start(self):
        for c in self.channels:
            t=threading.Thread(target=self._work,args=(c,),daemon=True); t.start(); self.threads.append(t)
        t=threading.Thread(target=self._cleaner,daemon=True); t.start(); self.threads.append(t)

    def stop(self):
        with self.cv: self.stopping=True; self.cv.notify_all()
        for t in self.threads: t.join(timeout=2)

    def _used(self): return sum(len(j.get('payload') or b'') + len(j.get('result') or b'') for j in self.jobs.values())

    def _drop(self,j,state):
        j.update(state=state,payload=None,result=None,finished=self.clock())

    def submit(self,owner,key,parameters):
        payload=json.dumps(parameters,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        fingerprint=hashlib.sha256(payload).hexdigest()
        model=parameters['model']
        with self.cv:
            self.cleanup()
            previous=self.keys.get((owner,key))
            if previous:
                j=self.jobs[previous]
                if j['fingerprint']!=fingerprint: raise Rejected(409,'idempotency_conflict')
                return self._view(j), False
            if model not in self.queues: raise Rejected(400,'unsupported_model')
            if any(j['owner']==owner and j['state'] in ('queued','running') for j in self.jobs.values()):
                raise Rejected(429,'one_unfinished_job_per_session')
            if len(self.queues[model])>=self.queue_limit: raise Rejected(429,'queue_full')
            if self._used()+len(payload)>self.byte_limit or len(self.jobs)>=self.max_jobs: raise Rejected(503,'memory_capacity')
            ident=secrets.token_hex(16)
            j=dict(id=ident,owner=owner,key=key,fingerprint=fingerprint,model=model,state='queued',
                   created=self.clock(),payload=payload,result=None,channel=None)
            self.jobs[ident]=j; self.keys[(owner,key)]=ident; self.queues[model].append(ident)
            self.cv.notify_all()
            return self._view(j), True

    def _owned(self,owner,ident):
        self.cleanup()
        j=self.jobs.get(ident)
        if j is None or j['owner']!=owner: raise Rejected(404,'unknown_job')
        return j

    def _view(self,j):
        result={k:j[k] for k in ('id','model','state','channel')}
        result['elapsed_seconds']=round(self.clock()-j['created'],3)
        if j['state']=='queued': result['ahead']=list(self.queues[j['model']]).index(j['id'])
        if 'started' in j: result['queue_seconds']=round(j['started']-j['created'],3)
        if 'execution_seconds' in j: result['execution_seconds']=j['execution_seconds']
        if 'error' in j: result['error']=j['error']
        if j.get('meta'): result['result']=j['meta']
        if j['state']=='completed': result['expires_in_seconds']=max(0,round(j['finished']+self.result_ttl-self.clock(),1))
        return result

    def status(self,owner,ident):
        with self.cv: return self._view(self._owned(owner,ident))

    def lookup(self,owner,key):
        with self.cv:
            self.cleanup()
            ident=self.keys.get((owner,key))
            if not ident: raise Rejected(404,'unknown_request')
            return self._view(self._owned(owner,ident))

    def result(self,owner,ident):
        with self.cv:
            j=self._owned(owner,ident)
            if j['state']!='completed': raise Rejected(410 if j['state'] in ('released','expired','cancelled') else 409,'result_unavailable')
            return j['result']

    def ack(self,owner,ident):
        with self.cv:
            j=self._owned(owner,ident)
            if j['state']=='released': return self._view(j)
            if j['state']!='completed': raise Rejected(409,'not_completed')
            self._drop(j,'released'); return self._view(j)

    def cancel(self,owner,ident):
        with self.cv:
            j=self._owned(owner,ident)
            if j['state']=='queued':
                self.queues[j['model']].remove(ident); self._drop(j,'cancelled')
            elif j['state']=='running':
                # Do not release the GPU slot while inference may still be running.
                j['cancel_requested']=True
            elif j['state']=='completed': self._drop(j,'cancelled')
            v=self._view(j); v['cancel_requested']=j.get('cancel_requested',False); return v

    def cleanup(self):
        with self.cv:
            now=self.clock()
            for ident,j in list(self.jobs.items()):
                if j['state']=='queued' and now-j['created']>=self.queue_ttl:
                    self.queues[j['model']].remove(ident); self._drop(j,'expired')
                elif j['state']=='completed' and now-j['finished']>=self.result_ttl: self._drop(j,'expired')
                elif j['state'] in ('released','expired','cancelled','failed') and now-j['finished']>=self.metadata_ttl:
                    self.keys.pop((j['owner'],j['key']),None); del self.jobs[ident]

    def metrics(self):
        with self.cv:
            self.cleanup()
            return {'content_bytes':self._used(),'retained_jobs':len(self.jobs),
                    'models':{m:{'queued':len(q),'running':sum(j['state']=='running' and j['model']==m for j in self.jobs.values()),
                                 'channels':[{'id':c['id'],'state':c['state']} for c in self.channels if c['model']==m]} for m,q in self.queues.items()}}

    def _cleaner(self):
        while True:
            with self.cv:
                if self.stopping: return
                self.cleanup(); self.cv.wait(0.5)

    def _work(self,c):
        while True:
            with self.cv:
                self.cleanup()
                if self.stopping: return
                if c['state']=='quarantined' or not self.queues[c['model']]:
                    self.cv.wait(0.2); continue
            try: is_ready=self.ready(c)
            except Exception: is_ready=False
            if not is_ready:
                with self.cv: c['state']='unavailable'; self.cv.wait(0.5)
                continue
            with self.cv:
                self.cleanup()
                if self.stopping: return
                if not self.queues[c['model']]: c['state']='idle'; continue
                j=self.jobs[self.queues[c['model']].popleft()]
                j.update(state='running',started=self.clock(),channel=c['id'])
                c.update(state='running',job=j['id'])
                payload=j['payload']
            result=meta=None
            try:
                execution_channel = dict(c, owner=j['owner'], job_id=j['id'])
                result,meta=self.runner(execution_channel,payload)
                if not isinstance(result,bytes) or not result.startswith(b'\x89PNG\r\n\x1a\n'): raise ValueError('Invalid worker PNG')
                with self.cv:
                    j['payload']=None
                    if j.get('cancel_requested'): self._drop(j,'cancelled')
                    elif self._used()+len(result)>self.byte_limit: self._drop(j,'failed'); j['error']='result_memory_capacity'
                    else: j.update(state='completed',result=result,meta=meta,finished=self.clock())
            except BusyBeforeAccept:
                with self.cv:
                    if j.get('cancel_requested'): self._drop(j,'cancelled')
                    else:
                        j.update(state='queued',channel=None); j.pop('started',None)
                        self.queues[j['model']].appendleft(j['id'])
                    self.cv.wait(0.2)
            except Exception as exc:
                with self.cv:
                    self._drop(j,'failed')
                    j['error']='execution_uncertain_no_retry' if isinstance(exc,UncertainExecution) else 'worker_failed'
                    if isinstance(exc,UncertainExecution): c['state']='quarantined'
            finally:
                payload=result=meta=None
                with self.cv:
                    if 'started' in j: j['execution_seconds']=round(self.clock()-j['started'],3)
                    if c['state']!='quarantined': c['state']='idle'
                    c['job']=None; self.cv.notify_all()
