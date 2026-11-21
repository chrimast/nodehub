<script setup lang="ts">
import {ref,onMounted,onUnmounted,defineAsyncComponent} from 'vue'

import Cleanup from './Cleanup.vue'
const TrafficDisplay=defineAsyncComponent(()=>import('./TrafficDisplay.vue'))
const PanelUpdates=defineAsyncComponent(()=>import('./PanelUpdates.vue'))
const Notifications=defineAsyncComponent(()=>import('./Notifications.vue'))

const emit=defineEmits(['branding'])
const originsText=ref(''),agentOriginsText=ref('')
const Topology=defineAsyncComponent(()=>import('./Topology.vue'))
type Row=Record<string,any>
const props=defineProps<{api:(p:string,m?:string,b?:any)=>Promise<any>,hosts:Row[],changePassword:(current:string,password:string,confirmation:string)=>Promise<void>}>()
const tab=ref('domain'),site=ref<Row>({domain:'',https:true}),error=ref(''),notice=ref(''),busy=ref(false),rows=ref<Row[]>([])
const currentPassword=ref(''),newPassword=ref(''),confirmPassword=ref('')
let statusTimer:ReturnType<typeof setTimeout>|undefined,disposed=false
async function refreshGatewayStatus(){const fresh=await props.api('site');if(disposed)return;for(const key of ['gateway_domain','gateway_state','gateway_https','gateway_checked_at','share_gateway_domain','share_gateway_state','share_gateway_checked_at'])site.value[key]=fresh[key]}
function pollGatewayStatus(remaining=24){clearTimeout(statusTimer);if(!remaining||disposed)return;statusTimer=setTimeout(async()=>{try{await refreshGatewayStatus();if(site.value.gateway_state==='pending'||site.value.share_gateway_state==='pending')pollGatewayStatus(remaining-1)}catch{pollGatewayStatus(remaining-1)}},5000)}
async function run(fn:()=>Promise<void>){busy.value=true;error.value='';try{await fn()}catch(e){error.value=e instanceof Error?e.message:String(e)}finally{busy.value=false}}
onMounted(()=>void run(async()=>{site.value=await props.api('site');originsText.value=(site.value.api_origins??[]).join('\n');agentOriginsText.value=(site.value.agent_origins??[]).join('\n');if(site.value.gateway_state==='pending'||site.value.share_gateway_state==='pending')pollGatewayStatus()}))
onUnmounted(()=>{disposed=true;clearTimeout(statusTimer)})
function clearPasswords(){currentPassword.value=newPassword.value=confirmPassword.value=''}
function select(v:string){clearPasswords();error.value=notice.value='';tab.value=v;if(['audit','tasks'].includes(v))void run(async()=>rows.value=await props.api(v))}
function savePassword(){
 if(busy.value)return
 notice.value=''
 void run(async()=>{
  if(newPassword.value!==confirmPassword.value)throw new Error('两次输入的新密码不一致')
  await props.changePassword(currentPassword.value,newPassword.value,confirmPassword.value)
  clearPasswords()
  notice.value='密码已修改'
 })
}
function save(){void run(async()=>{await props.api('site','PUT',{domain:site.value.domain,https:site.value.https,title:site.value.title,ip_origin:site.value.ip_origin,share_domain:site.value.share_domain,share_https:site.value.share_https,agent_origins:agentOriginsText.value.split(/\r?\n/).map(v=>v.trim()).filter(Boolean),api_origins:originsText.value.split(/\r?\n/).map(v=>v.trim()).filter(Boolean)});site.value=await props.api('site');emit('branding');notice.value='域名设置已保存，网关同步器会检查 DNS、申请证书并回报结果。通常约一分钟，完成前请勿使用新域名分享；无需重启主机。';pollGatewayStatus()})}

</script>
<template><nav class="subnav"><button v-for="t in [{id:'domain',name:'站点与域名'},{id:'display',name:'显示与单位'},{id:'version',name:'后台版本'},{id:'password',name:'修改密码'},{id:'notifications',name:'通知与 Telegram'},{id:'tools',name:'证书 / 反代 / 兼容目录'},{id:'audit',name:'审计'},{id:'tasks',name:'历史任务'},{id:'cleanup',name:'验收清理'}]" :key="t.id" class="secondary" :class="{active:tab===t.id}" @click="select(t.id)">{{t.name}}</button></nav><p v-if="error" role="alert" class="error">{{error}}</p><p v-if="notice" role="status" class="notice">{{notice}}</p>
<section v-if="tab==='domain'" class="panel publish-card"><h2>域名与 HTTPS</h2><p class="muted">域名可留空，使用配置的 IP + 端口访问。订阅域名可独立设置，留空时依次使用主站域名、IP 入口。更换域名前请配置 DNS。</p><form @submit.prevent="save"><div class="fields"><label>网站名称<input v-model="site.title" required maxlength="30" placeholder="星枢"></label><label>控制台域名（可选）<input v-model="site.domain" placeholder="vps.example.com"></label><label class="check-label"><input v-model="site.https" type="checkbox">启用 HTTPS 与自动证书</label><label>IP 访问入口<input v-model="site.ip_origin" placeholder="http://45.129.8.122:18080" required></label><label>独立订阅域名（可选）<input v-model="site.share_domain" placeholder="sub.example.com"></label><label class="check-label"><input v-model="site.share_https" type="checkbox">订阅域名启用 HTTPS</label></div><label class="wide-field">Agent 控制地址（可选，每行一个，最多 4 个）<textarea v-model="agentOriginsText" rows="3" placeholder="https://agent.example.com"></textarea><small>留空跟随控制台域名。独立地址须事先配置有效 HTTPS 并反代至本控制端 API，不会自动创建此地址的网关。在线 Agent 0.23.13+ 验证身份后切换；换域名前保留旧入口，等所有主机完成切换后再停用。已失联可在“连接 / 恢复”生成修复命令，保留凭据与节点。</small></label><label class="wide-field">独立前端允许来源（每行一个 HTTPS Origin，最多 8 个）<textarea v-model="originsText" rows="3" placeholder="https://panel.example.com"></textarea><small>不填写路径，不使用通配符；本机开发可填 http://localhost:5173。跨域仍需密钥权限。</small></label><p>网关当前：{{site.gateway_domain||'尚未回报'}} · {{site.gateway_state||'等待同步'}} · {{site.gateway_https?'HTTPS':'HTTP'}}</p><p>订阅网关：{{site.share_gateway_domain||'继承主站'}} · {{site.share_gateway_state==='pending'?'等待网关同步':site.share_gateway_state==='error'?'同步失败，请检查 DNS、证书和网关日志':site.share_gateway_state||'等待同步'}}</p><p v-if="site.gateway_state==='pending'||site.share_gateway_state==='pending'" class="muted" role="status">设置已保存，正在等待网关；状态会自动刷新，不需要重启主机。</p><button type="button" class="secondary" :disabled="busy" @click="run(refreshGatewayStatus)">刷新网关状态</button><button class="primary" :disabled="busy">保存域名设置</button></form></section>
<TrafficDisplay v-else-if="tab==='display'" :api="api" @saved="emit('branding')"/>
<PanelUpdates v-else-if="tab==='version'" :api="api"/><Notifications v-else-if="tab==='notifications'" :api="api"/><Cleanup v-else-if="tab==='cleanup'" :api="api"/>
<section v-else-if="tab==='password'" class="panel publish-card">
 <h2>修改管理员密码</h2>
 <form class="password-form" @submit.prevent="savePassword">
  <label>当前密码<input v-model="currentPassword" type="password" autocomplete="current-password" required :disabled="busy"></label>
  <label>新密码<input v-model="newPassword" type="password" autocomplete="new-password" minlength="8" maxlength="128" required :disabled="busy"></label>
  <label>确认新密码<input v-model="confirmPassword" type="password" autocomplete="new-password" required :disabled="busy"></label>
  <button class="primary" :disabled="busy">{{busy?'保存中…':'保存新密码'}}</button>
 </form>
</section>
<Topology v-else-if="tab==='tools'" :api="api" :hosts="hosts" tools-only/>
<section v-else class="panel publish-card"><h2>{{tab==='audit'?'操作审计':'历史任务'}}</h2><div v-for="r in rows" :key="r.id??r.sequence" class="setting-row"><div><b>{{r.name??r.action??r.id}}</b><small class="block-muted">{{r.state??r.actor??r.host_id}} · {{r.result??r.created_at}}</small></div><button v-if="tab==='tasks'&&r.state==='queued'" class="text-button" @click="run(async()=>{await api('tasks/'+r.id+'/cancel','POST',{});rows=await api('tasks')})">取消任务</button></div><p v-if="!rows.length" class="console-empty">暂无记录</p></section></template>
<style scoped>.password-form{display:grid;gap:10px;max-width:360px}.password-form button{justify-self:start}</style>
