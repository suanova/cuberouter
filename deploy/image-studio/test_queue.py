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

import json, threading, time, unittest
from queue_core import Dispatcher, Rejected, UncertainExecution, BusyBeforeAccept

PNG=b'\x89PNG\r\n\x1a\n'+b'fixture'
def params(model='2512',prompt='test'): return {'model':model,'prompt':prompt}
def wait_for(fn,timeout=5):
    end=time.monotonic()+timeout
    while time.monotonic()<end:
        if fn(): return
        time.sleep(.01)
    raise AssertionError('Condition did not occur')

class QueueTests(unittest.TestCase):
    def make(self,**kw):
        d=Dispatcher([{'id':'one','model':'2512'}],lambda c,p:(PNG,{}),**kw)
        self.addCleanup(d.stop); return d

    def test_qwen21_shared_queue_distributes_to_eight_independent_slots(self):
        gate=threading.Event()
        guard=threading.Lock(); active=set(); calls=[]; peak=[0]
        def run(channel,payload):
            with guard:
                self.assertNotIn(channel['id'],active)
                active.add(channel['id']); peak[0]=max(peak[0],len(active))
                calls.append(json.loads(payload)['prompt'])
            if not gate.wait(5): raise RuntimeError('test gate timeout')
            with guard: active.remove(channel['id'])
            return PNG,{}
        d=Dispatcher([{'id':f'gpu-{i}','model':'2.1'} for i in range(8)],run,queue_limit=20)
        self.addCleanup(d.stop); self.addCleanup(gate.set); d.start()
        submitted=[]
        for i in range(12):
            owner=f'user-{i}'
            job,_=d.submit(owner,owner,params('2.1',owner)); submitted.append((owner,job['id']))
        wait_for(lambda:len(active)==8)
        self.assertEqual(d.metrics()['models']['2.1']['running'],8)
        self.assertEqual(d.metrics()['models']['2.1']['queued'],4)
        gate.set()
        wait_for(lambda:all(d.status(owner,job)['state']=='completed' for owner,job in submitted))
        self.assertEqual(peak[0],8)
        self.assertCountEqual(calls,[f'user-{i}' for i in range(12)])
        for owner,job in submitted: d.ack(owner,job)
        self.assertEqual(d.metrics()['content_bytes'],0)

    def test_eight_channels_per_model_and_no_double_use(self):
        gate=threading.Event(); self.addCleanup(gate.set)
        guard=threading.Lock(); active=set(); peak=[0]; calls=[]
        channels=[{'id':f'{model}-{i}','model':model} for model in ('2511','2512') for i in range(8)]
        def run(c,payload):
            p=json.loads(payload)
            with guard:
                self.assertNotIn(c['id'],active); self.assertEqual(c['model'],p['model'])
                active.add(c['id']); peak[0]=max(peak[0],len(active)); calls.append(p['prompt'])
            if not gate.wait(5): raise RuntimeError('test gate timeout')
            with guard: active.remove(c['id'])
            return PNG,{}
        d=Dispatcher(channels,run,queue_limit=20); self.addCleanup(d.stop); d.start()
        submitted=[]
        for model in ('2511','2512'):
            for i in range(12):
                owner=f'{model}-{i}'; j,_=d.submit(owner,owner,params(model,owner)); submitted.append((owner,j['id']))
        wait_for(lambda:len(active)==16)
        self.assertEqual(sum(x['queued'] for x in d.metrics()['models'].values()),8)
        gate.set(); wait_for(lambda:all(d.status(o,j)['state']=='completed' for o,j in submitted))
        self.assertEqual(peak[0],16); self.assertEqual(len(calls),24); self.assertEqual(len(set(calls)),24)
        for owner,j in submitted: d.ack(owner,j)
        self.assertEqual(d.metrics()['content_bytes'],0)

    def test_owner_checks_cover_status_result_ack_cancel(self):
        d=self.make(); d.start(); j,_=d.submit('a','key',params()); ident=j['id']
        wait_for(lambda:d.status('a',ident)['state']=='completed')
        for action in (d.status,d.result,d.ack,d.cancel):
            with self.assertRaises(Rejected) as error: action('b',ident)
            self.assertEqual(error.exception.status,404)
        self.assertEqual(d.result('a',ident),PNG)
        d.ack('a',ident); self.assertEqual(d.metrics()['content_bytes'],0)
        with self.assertRaises(Rejected) as error: d.result('a',ident)
        self.assertEqual(error.exception.status,410)

    def test_idempotency_and_one_job_across_models(self):
        d=Dispatcher([{'id':'a','model':'2512'},{'id':'b','model':'2511'}],lambda c,p:(PNG,{}))
        self.addCleanup(d.stop)
        j,created=d.submit('a','key',params()); self.assertTrue(created)
        same,created=d.submit('a','key',params()); self.assertFalse(created); self.assertEqual(j['id'],same['id'])
        with self.assertRaises(Rejected) as e:d.submit('a','key',params(prompt='changed'))
        self.assertEqual(e.exception.code,'idempotency_conflict')
        with self.assertRaises(Rejected) as e:d.submit('a','second',params('2511'))
        self.assertEqual(e.exception.code,'one_unfinished_job_per_session')

    def test_queue_full_cancel_and_memory_limit(self):
        d=self.make(queue_limit=1); j,_=d.submit('a','a',params())
        with self.assertRaises(Rejected) as e:d.submit('b','b',params())
        self.assertEqual(e.exception.code,'queue_full')
        d.cancel('a',j['id']); self.assertEqual(d.metrics()['content_bytes'],0)
        d.submit('b','b',params())
        tiny=self.make(byte_limit=5)
        with self.assertRaises(Rejected) as e:tiny.submit('x','x',params())
        self.assertEqual(e.exception.code,'memory_capacity')

    def test_result_and_queue_expiry(self):
        clock=[0]; d=self.make(clock=lambda:clock[0],queue_ttl=2,result_ttl=2,metadata_ttl=2)
        j,_=d.submit('a','a',params()); clock[0]=3
        self.assertEqual(d.status('a',j['id'])['state'],'expired'); self.assertEqual(d.metrics()['content_bytes'],0)
        d.start(); j,_=d.submit('b','b',params()); wait_for(lambda:d.status('b',j['id'])['state']=='completed')
        clock[0]=6; self.assertEqual(d.status('b',j['id'])['state'],'expired'); self.assertEqual(d.metrics()['content_bytes'],0)
        clock[0]=9; d.cleanup(); self.assertEqual(d.metrics()['retained_jobs'],0)

    def test_unavailable_channel_does_not_block_healthy_channel(self):
        d=Dispatcher([{'id':'offline','model':'2512'},{'id':'ok','model':'2512'}],lambda c,p:(PNG,{}),ready=lambda c:c['id']=='ok')
        self.addCleanup(d.stop); d.start(); j,_=d.submit('a','a',params())
        wait_for(lambda:d.status('a',j['id'])['state']=='completed'); self.assertEqual(d.status('a',j['id'])['channel'],'ok')

    def test_transport_uncertainty_quarantines_no_retry(self):
        calls=[]
        def run(c,p): calls.append(p); raise UncertainExecution()
        d=Dispatcher([{'id':'a','model':'2512'}],run); self.addCleanup(d.stop); d.start()
        j,_=d.submit('a','a',params()); wait_for(lambda:d.status('a',j['id'])['state']=='failed')
        self.assertEqual(len(calls),1); self.assertEqual(d.channels[0]['state'],'quarantined')
        self.assertEqual(d.status('a',j['id'])['error'],'execution_uncertain_no_retry')
        self.assertEqual(d.metrics()['content_bytes'],0)

    def test_busy_before_accept_retry_safe(self):
        calls=[]
        def run(c,p):
            calls.append(p)
            if len(calls)==1: raise BusyBeforeAccept()
            return PNG,{}
        d=Dispatcher([{'id':'a','model':'2512'}],run); self.addCleanup(d.stop); d.start()
        j,_=d.submit('a','a',params()); wait_for(lambda:d.status('a',j['id'])['state']=='completed')
        self.assertEqual(len(calls),2); self.assertEqual(len(d.jobs),1)

    def test_running_cancel_holds_slot_and_discards_output(self):
        gate=threading.Event(); self.addCleanup(gate.set)
        d=Dispatcher([{'id':'a','model':'2512'}],lambda c,p:(gate.wait(5) and PNG,{}))
        self.addCleanup(d.stop); d.start(); j,_=d.submit('a','a',params()); wait_for(lambda:d.status('a',j['id'])['state']=='running')
        d.cancel('a',j['id']); self.assertEqual(d.status('a',j['id'])['state'],'running')
        gate.set(); wait_for(lambda:d.status('a',j['id'])['state']=='cancelled'); self.assertEqual(d.metrics()['content_bytes'],0)

    def test_duplicate_physical_endpoint_rejected(self):
        with self.assertRaises(ValueError): Dispatcher([{'id':'a','model':'2512','url':'same'},{'id':'b','model':'2512','url':'same'}],None)

if __name__=='__main__': unittest.main(verbosity=2)
