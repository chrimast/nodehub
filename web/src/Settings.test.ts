// @vitest-environment happy-dom
import {afterEach,it,expect,vi} from 'vitest'
import {mount,flushPromises,type VueWrapper} from '@vue/test-utils'
import Settings from './Settings.vue'
let w:VueWrapper
afterEach(()=>w?.unmount())
it('replaces integration and firewall settings with a password form',async()=>{
 const api=vi.fn(async()=>({})),changePassword=vi.fn().mockRejectedValueOnce(Error('当前密码验证失败')).mockResolvedValue(undefined)
 w=mount(Settings,{props:{api,hosts:[],changePassword}});await flushPromises()
 const tabs=w.get('nav');expect(tabs.text()).not.toContain('API 与集成');expect(tabs.text()).not.toContain('防火墙')
 await tabs.findAll('button').find(b=>b.text()==='修改密码')!.trigger('click')
 const form=w.get('form'),[current,password,confirmation]=form.findAll('input[type=password]')
 await current!.setValue('old-fixture');await password!.setValue('new-fixture');await confirmation!.setValue('different-fixture')
 await form.trigger('submit');expect(changePassword).not.toHaveBeenCalled()
 await confirmation!.setValue('new-fixture');await form.trigger('submit');await flushPromises()
 expect(w.get('[role=alert]').text()).toBe('当前密码验证失败')
 await form.trigger('submit');await flushPromises();expect(changePassword).toHaveBeenLastCalledWith('old-fixture','new-fixture','new-fixture')
 expect(w.get('[role=status]').text()).toContain('密码已修改')
 expect(form.findAll('input').map(i=>i.element.value)).toEqual(['','',''])
})
it('saves domain and HTTPS without claiming the gateway already changed',async()=>{const api=vi.fn(async(p:string)=>p==='site'?{domain:'vps.example.com',https:true,gateway_state:'pending'}:[]);w=mount(Settings,{props:{api,hosts:[],changePassword:vi.fn()}});await flushPromises();await w.get('input[placeholder="vps.example.com"]').setValue('new.example.com');await w.get('form').trigger('submit');await flushPromises();expect(api).toHaveBeenCalledWith('site','PUT',expect.objectContaining({domain:'new.example.com',https:true,api_origins:[]}));expect(w.text()).toContain('网关同步器会检查 DNS')})
it('refreshes gateway status without discarding unsaved fields',async()=>{
 let active=false
 const api=vi.fn(async(p:string)=>p==='site'?{domain:'vps.example.com',https:true,share_domain:'sub.example.com',share_https:true,gateway_state:'active',share_gateway_domain:active?'sub.example.com':'',share_gateway_state:active?'active':'pending'}:[])
 w=mount(Settings,{props:{api,hosts:[],changePassword:vi.fn()}});await flushPromises()
 await w.get('input[placeholder="vps.example.com"]').setValue('draft.example.com')
 expect(w.text()).toContain('等待网关同步')
 active=true;await w.findAll('button').find(b=>b.text()==='刷新网关状态')!.trigger('click');await flushPromises()
 expect((w.get('input[placeholder="vps.example.com"]').element as HTMLInputElement).value).toBe('draft.example.com')
 expect(w.text()).toContain('sub.example.com · active')
})
