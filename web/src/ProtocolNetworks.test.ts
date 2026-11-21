// @vitest-environment happy-dom
import {expect,it} from 'vitest'
import {mount} from '@vue/test-utils'
import {reactive} from 'vue'
import ProtocolLayers from './ProtocolLayers.vue'
it('labels the Vision option with its stored flow value',async()=>{
 const form=reactive({engine:'xray',kind:'vless',transport:'tcp',tls_mode:'tls',flow:''})
 const wrapper=mount(ProtocolLayers,{props:{form}})
 try{const option=wrapper.get('option[value="xtls-rprx-vision"]');expect(option.text()).toBe('xtls-rprx-vision');await wrapper.get('select:has(option[value="xtls-rprx-vision"])').setValue('xtls-rprx-vision');expect(form.flow).toBe('xtls-rprx-vision')}finally{wrapper.unmount()}
})
it('separates native SS listener networks from framing and preserves the selection',async()=>{
 const form={kind:'shadowsocks',engine:'sing-box',transport:'tcp',tls_mode:'none',listen_network:''}
 const wrapper=mount(ProtocolLayers,{props:{form,template:{layers:{'sing-box':[{transport:'tcp',security_modes:['none']}]},network_note:'SS 支持 TCP / UDP'}}})
 expect(wrapper.text()).toContain('不是仅 TCP')
 await wrapper.get('[aria-label="Shadowsocks 原生监听网络"]').setValue('udp')
 expect(form.listen_network).toBe('udp');expect(form.transport).toBe('tcp');expect(form.tls_mode).toBe('none')
 wrapper.unmount()
})

it('shows XHTTP padding with the official default and persists edits to the shared form',async()=>{
 const form=reactive({engine:'xray',kind:'vless',transport:'xhttp',tls_mode:'reality',path:'/video',ws_host:'',xhttp_padding:'100-1000',vless_encryption:false,flow:''})
 const wrapper=mount(ProtocolLayers,{props:{form,template:{layers:{xray:[{transport:'tcp',security_modes:['tls','reality']},{transport:'xhttp',security_modes:['tls','reality']}]}}}})
 try {
  const input=wrapper.get<HTMLInputElement>('input[placeholder="100-1000"]')
  const pattern=new RegExp(`^(?:${input.element.pattern})$`,'v')
  expect(input.element.value).toBe('100-1000')
  expect(pattern.test(input.element.value)).toBe(true)
  await input.setValue('50-500');expect(pattern.test(input.element.value)).toBe(true);expect(form.xhttp_padding).toBe('50-500')
  expect(pattern.test('100:1000')).toBe(false)
 } finally {wrapper.unmount()}
})
