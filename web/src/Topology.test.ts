// @vitest-environment happy-dom
import {afterEach,beforeEach,it,expect,vi,type Mock} from 'vitest'
import {mount,flushPromises,type VueWrapper} from '@vue/test-utils'
import Topology from './Topology.vue'

let w:VueWrapper,api:Mock<(p:string,m?:string,b?:any)=>Promise<any>>
const host={id:'a'.repeat(32),name:'VPS',address:'192.0.2.1',agent_version:'0.10.0'}
const service={id:'b'.repeat(32),host_id:host.id,name:'VMess entry',kind:'vmess',engine:'sing-box',enabled:false,version:2}
beforeEach(()=>{api=vi.fn(async(p,m='GET')=>{if(m!=='GET')return {pending_stop:true};if(p==='live')return [service];if(p==='live/'+service.id)return {...service,node_id:'n',certificate_id:'',next_id:'',spec:{kind:'vmess',engine:'sing-box',listen:'127.0.0.1',port:2080,address:'192.0.2.1',transport:'ws',path:'/kept',tls_mode:'none'},rendered:{inbounds:[],outbounds:[]}};if(p==='protocol-templates')return [{kind:'vless',name:'VLESS',certificate:true,engines:['sing-box','xray']},{kind:'vmess',name:'VMess',certificate:true,engines:['sing-box','xray']},{kind:'hysteria2',name:'HY2',certificate:true,engines:['sing-box']}];return []});w=mount(Topology,{props:{api,hosts:[host]}})})
afterEach(()=>{w.unmount();vi.useRealTimers();vi.restoreAllMocks()})
const button=(text:string)=>w.findAll('button').find(b=>b.text()===text)!
it('only shows recovery actions accepted for the protocol state',async()=>{
 api.mockImplementation(async p=>p==='live'?[service,{...service,id:'c'.repeat(32),delete_requested_at:'2026-11-21T00:00:00Z'}]:[])
 w.unmount();w=mount(Topology,{props:{api,hosts:[host]}});await flushPromises()
 const [normal,pending]=w.findAll('tbody tr')
 expect(normal.text()).toContain('重新生成加密配置')
 expect(normal.text()).not.toContain('强制清理并重同步')
 expect(pending.text()).toContain('强制清理并重同步')
 expect(pending.text()).not.toContain('重新生成加密配置')
})
it('filters protocols under the kernel without requiring a logical-node grouping',async()=>{await flushPromises();await button('添加协议').trigger('click');await flushPromises();const dialog=w.get('[role=dialog]');expect(dialog.text()).not.toContain('逻辑节点分组');await dialog.findAll('select')[1].setValue('xray');expect(dialog.findAll('select')[2].text()).toContain('VMess');expect(dialog.findAll('select')[2].text()).not.toContain('HY2')})
it('keeps existing transport and managed-user settings during an edit',async()=>{await flushPromises();await button('详情 / 编辑').trigger('click');await flushPromises();await w.get('form[role=dialog]').trigger('submit');await flushPromises();expect(api).toHaveBeenCalledWith('live/'+service.id,'PUT',expect.objectContaining({version:2,transport:'ws',path:'/kept',managed_users:false}))})
it('offers updated vendor candidates without Apple and applies the selected target and SNI',async()=>{await flushPromises();await button('添加协议').trigger('click');await flushPromises();const select=w.get('[aria-label="大厂 SNI 候选"]');expect(select.text()).not.toContain('Apple');expect(select.text()).toContain('Bing');expect(select.text()).toContain('AWS');expect(select.text()).toContain('NVIDIA');expect(select.text()).toContain('Samsung');await select.setValue('www.bing.com');const inputValues=w.get('[role="dialog"]').findAll('input').map((i:any)=>(i.element as HTMLInputElement).value);expect(inputValues.filter((v:string)=>v==='www.bing.com')).toHaveLength(2)})
it('loads same-prefix TLS names and selects one as target and SNI',async()=>{await flushPromises();api.mockImplementation(async(p:string,m='GET')=>{if(p==='reality-domain-scan')return {domains:['edge.example.net']};if(m!=='GET')return {pending_stop:true};if(p==='live')return [service];if(p==='protocol-templates')return [{kind:'vless',name:'VLESS',engines:['sing-box']}];return []});await button('添加协议').trigger('click');await flushPromises();await w.get('[aria-label="REALITY 目标 IP 网段"]').setValue('198.51.100.0/29');await button('扫描该网段 HTTPS 域名').trigger('click');await flushPromises();const found=w.get('[aria-label="同 IP 段域名候选"]');expect(found.text()).toContain('edge.example.net');await found.setValue('edge.example.net');const values=w.get('[role="dialog"]').findAll('input').map((i:any)=>(i.element as HTMLInputElement).value);expect(values.filter((v:string)=>v==='edge.example.net')).toHaveLength(2)})
it('keeps protocol editor at a narrower single-column layout',async()=>{await flushPromises();await button('添加协议').trigger('click');await flushPromises();expect(w.get('form[role="dialog"]').classes()).toContain('single-column-editor');expect(w.get('form[role="dialog"]').attributes('data-layout')).toBe('single-column')})
it('opens the batch protocol dialog when clicked',async()=>{await flushPromises();await button('批量添加协议').trigger('click');await flushPromises();expect(w.get('form[role="dialog"]').attributes('aria-label')).toBe('批量配置协议')})

it('shows sing-box latest service fields only for HY2 and TUIC',async()=>{api.mockImplementation(async(p:string,m='GET')=>{if(m!=='GET')return {pending_stop:true};if(p==='live')return [service];if(p==='protocol-templates')return [{kind:'vless',name:'VLESS',engines:['sing-box','xray']},{kind:'hysteria2',name:'HY2',engines:['sing-box']},{kind:'tuic',name:'TUIC',engines:['sing-box']}];return []});await button('添加协议').trigger('click');await flushPromises();await w.get('form[role="dialog"]').findAll('select').find((x:any)=>Array.from((x.element as HTMLSelectElement).options).some(o=>o.value==='hysteria2'))!.setValue('hysteria2');await flushPromises();expect(w.find('[aria-label="HY2 服务端混淆类型"]').exists()).toBe(true);expect(w.find('[aria-label="HY2 服务端 BBR profile"]').exists()).toBe(true);})

it('does not send native-only default options when adding a basic Xray VLESS node',async()=>{api.mockImplementation(async(p:string,m='GET')=>{if(m!=='GET')return {id:'new'};if(p==='live')return [service];if(p==='protocol-templates')return [{kind:'vless',name:'VLESS',engines:['sing-box','xray']}];return []});await flushPromises();await button('添加协议').trigger('click');await flushPromises();const d=w.get('[role="dialog"]');await d.findAll('select')[1].setValue('xray');await d.findAll('select')[2].setValue('vless');await flushPromises();await d.trigger('submit');await flushPromises();expect(api).toHaveBeenCalledWith('live','POST',expect.objectContaining({engine:'xray',kind:'vless'}));expect(w.text()).not.toContain('latest native options in this group require sing-box')})
it('confirms deletion and explains waiting for an Agent stop receipt',async()=>{await flushPromises();api.mockClear();await button('删除').trigger('click');expect(api).not.toHaveBeenCalled();await button('确认执行').trigger('click');await flushPromises();expect(api).toHaveBeenCalledWith('live/'+service.id,'DELETE');expect(w.text()).toContain('等待 Agent 停止回执')})
it('does not poll kernel catalogs in a hidden tab',async()=>{vi.useFakeTimers();await flushPromises();api.mockClear();vi.spyOn(document,'visibilityState','get').mockReturnValue('hidden');await vi.advanceTimersByTimeAsync(90000);expect(api).not.toHaveBeenCalled()})

it('suggests a high proxy port without reusing a known port', async()=>{
 await flushPromises()
 await button('添加协议').trigger('click');await flushPromises()
 const port=Number((w.get('form[role="dialog"] input[type="number"][min="1024"][max="65535"]').element as HTMLInputElement).value)
 expect(port).toBeGreaterThanOrEqual(20000);expect(port).toBeLessThanOrEqual(49151)
})


it('submits XHTTP padding when creating and editing a protocol',async()=>{
 const node={...service,kind:'vless',engine:'xray'}
 const templates=[{kind:'vless',name:'VLESS',engines:['sing-box','xray'],layers:{xray:[{transport:'tcp',security_modes:['reality']},{transport:'xhttp',security_modes:['reality']}]}}]
 api.mockImplementation(async(p,m='GET')=>{
  if(m!=='GET')return {id:node.id}
  if(p==='live')return [node]
  if(p==='protocol-templates')return templates
  if(p==='live/'+node.id)return {...node,spec:{kind:'vless',engine:'xray',transport:'xhttp',tls_mode:'reality',xhttp_padding:'50-500'}}
  return []
 })
 w.unmount();w=mount(Topology,{props:{api,hosts:[host]}})
 await flushPromises();await button('添加协议').trigger('click');await flushPromises()
 const dialog=w.get('form[role="dialog"]')
 await dialog.findAll('select').find(s=>s.find('option[value="xray"]').exists())!.setValue('xray')
 await dialog.findAll('select').find(s=>s.find('option[value="xhttp"]').exists())!.setValue('xhttp')
 expect(dialog.get<HTMLInputElement>('input[placeholder="100-1000"]').element.value).toBe('100-1000')
 await dialog.trigger('submit');await flushPromises()
 expect(api).toHaveBeenCalledWith('live','POST',expect.objectContaining({xhttp_padding:'100-1000'}))
 await button('详情 / 编辑').trigger('click');await flushPromises()
 const padding=w.get<HTMLInputElement>('input[placeholder="100-1000"]')
 expect(padding.element.value).toBe('50-500')
 await padding.setValue('200-500');await w.get('form[role="dialog"]').trigger('submit');await flushPromises()
 expect(api).toHaveBeenCalledWith('live/'+node.id,'PUT',expect.objectContaining({xhttp_padding:'200-500'}))
})
