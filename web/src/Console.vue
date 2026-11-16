<script setup lang="ts">
import {ref,computed,watch,onUnmounted,onMounted,defineAsyncComponent,provide} from 'vue'
import Hosts from './Hosts.vue'
import {trafficUnitKey,normalizeTrafficUnit,type TrafficUnit} from './trafficUnits'
const trafficUnit=ref<TrafficUnit>('GB');provide(trafficUnitKey,trafficUnit)
const Traffic=defineAsyncComponent(()=>import('./Traffic.vue'))
const Topology=defineAsyncComponent(()=>import('./Topology.vue'))
const Users=defineAsyncComponent(()=>import('./Access.vue'))
const Subscriptions=defineAsyncComponent(()=>import('./Subscriptions.vue'))
const Policies=defineAsyncComponent(()=>import('./Policies.vue'))
const Settings=defineAsyncComponent(()=>import('./Settings.vue'))
const hostname=window.location.hostname
const token=ref(sessionStorage.getItem('nodehub-admin-token')??''),connected=ref(false),mustChange=ref(false),newPassword=ref(''),confirmPassword=ref(''),busy=ref(false),error=ref(''),page=ref('overview'),hosts=ref<any[]>([]),selectedHost=ref(''),selectedUser=ref('')
const pages=[{id:'hosts',name:'服务器节点',sub:'多 VPS · Agent 接入',icon:'▣'},{id:'nodes',name:'协议配置',sub:'入站 · 出站 · 内核',icon:'◈'},{id:'users',name:'用户权限',sub:'用户 · 用户组',icon:'♙'},{id:'policies',name:'客户端策略',sub:'策略组 · 规则 · 自动转换',icon:'⇄'},{id:'subscriptions',name:'订阅分享',sub:'用户 · 有效期 · 链接',icon:'↗'}]
const current=computed(()=>page.value==='overview'?{name:'控制台概览',sub:'主机状态 · 流量 · 配额 · 访问记录'}:page.value==='usage'?{name:'用户使用记录',sub:'用量与连接元数据'}:pages.find(p=>p.id===page.value)??{name:'系统设置',sub:'域名、证书、反代与防火墙'})
function preference(){try{return localStorage.getItem('nodehub-theme')??'light'}catch{return 'light'}}
const theme=ref(preference()),media=window.matchMedia('(prefers-color-scheme: dark)'),dark=ref(media.matches)
const changed=(e:MediaQueryListEvent)=>dark.value=e.matches;media.addEventListener('change',changed)
watch(theme,v=>{try{localStorage.setItem('nodehub-theme',v)}catch{}})
const resolved=computed(()=>theme.value==='system'?(dark.value?'dark':'light'):theme.value)
let epoch=0,abort=new AbortController(),timer:ReturnType<typeof setTimeout>,alive=true
async function api(path:string,method='GET',body?:unknown){
 const session=epoch
 const r=await fetch('/api/v1/'+path,{method,headers:{Authorization:'Bearer '+token.value,'Content-Type':'application/json'},credentials:'same-origin',signal:AbortSignal.any([abort.signal,AbortSignal.timeout(12000)]),body:body===undefined?undefined:JSON.stringify(body)})
 const data=await r.json();if(session!==epoch)throw new Error('会话已结束')
 if(!r.ok)throw new Error(data.error??'请求失败');return data
}
function schedule(){clearTimeout(timer);if(connected.value)timer=setTimeout(()=>{if(document.visibilityState==='visible')void refresh();else schedule()},30000)}
async function refresh(){const session=epoch;try{const data=await api('hosts');if(session!==epoch||!alive)return;hosts.value=data;void branding();error.value=''}catch(e){if(session===epoch)error.value=String(e)}finally{if(session===epoch)schedule()}}
async function login(){if(busy.value)return;busy.value=true;error.value='';const session=epoch;try{const state=await api('session');if(session!==epoch||!alive)return;sessionStorage.setItem('nodehub-admin-token',token.value);connected.value=true;mustChange.value=!!state.must_change_password;if(!mustChange.value){hosts.value=await api('hosts');schedule()}}catch(e){if(session===epoch)error.value=String(e)}finally{if(session===epoch)busy.value=false}}
async function changePassword(){if(busy.value)return;busy.value=true;error.value='';try{if(newPassword.value!==confirmPassword.value)throw new Error('两次输入的密码不一致');await api('admin/password','POST',{password:newPassword.value,confirm_password:confirmPassword.value});token.value=newPassword.value;sessionStorage.setItem('nodehub-admin-token',token.value);newPassword.value='';confirmPassword.value='';mustChange.value=false;hosts.value=await api('hosts');schedule()}catch(e){error.value=String(e)}finally{busy.value=false}}
function logout(){epoch++;abort.abort();abort=new AbortController();clearTimeout(timer);connected.value=false;mustChange.value=false;token.value='';sessionStorage.removeItem('nodehub-admin-token');newPassword.value='';hosts.value=[];selectedUser.value='';selectedHost.value='';page.value='overview';busy.value=false}
function configure(id:string){selectedHost.value=id;page.value='nodes'}
function usage(id:string){selectedUser.value=id;page.value='usage'}
function share(id:string){selectedUser.value=id;page.value='subscriptions'}
onUnmounted(()=>{alive=false;clearTimeout(timer);abort.abort();media.removeEventListener('change',changed)})
const siteTitle=ref('nodehub')
async function branding(){try{const r=await fetch('/api/v1/branding',{signal:AbortSignal.timeout(5000)});if(r.ok){const b=await r.json();siteTitle.value=b.title||'nodehub';trafficUnit.value=normalizeTrafficUnit(b.traffic_unit);document.title=siteTitle.value+' · VPS 节点控制台'}}catch{}}
onMounted(()=>{void branding();if(token.value)void login()})
</script>
<template>
<div class="console-root" :data-theme="resolved">
 <form v-if="!connected" class="entry" @submit.prevent="login"><div class="entry-brand"><b>◆</b> {{siteTitle}}<span>VPS NODE CONTROL</span></div><h1>一个面板，管理所有节点。</h1><p>连接 Agent，配置协议，管理用户权限并安全分享订阅。</p><label>管理员密码<input v-model="token" type="password" autocomplete="current-password" required placeholder="首次登录使用 admin"></label><p v-if="error" role="alert" class="error">{{error}}</p><button class="primary" :disabled="busy">{{busy?'登录中…':'登录控制台'}}</button></form>
 <form v-else-if="mustChange" class="entry" @submit.prevent="changePassword"><div class="entry-brand"><b>◆</b> {{siteTitle}}<span>FIRST LOGIN SECURITY</span></div><h1>请先修改管理员密码。</h1><p>首次登录密码仅用于初始化，修改后才能继续使用控制台。</p><label>新管理员密码<input v-model="newPassword" type="password" autocomplete="new-password" minlength="8" required placeholder="至少 8 位"></label><label>再次输入新密码<input v-model="confirmPassword" type="password" autocomplete="new-password" minlength="8" required placeholder="请再次输入密码"></label><p v-if="newPassword&&confirmPassword&&newPassword!==confirmPassword" role="alert" class="error">两次输入的密码不一致</p><p v-if="error" role="alert" class="error">{{error}}</p><button class="primary" :disabled="busy||newPassword!==confirmPassword">{{busy?'保存中…':'保存并进入控制台'}}</button></form>
 <div v-else class="console-layout">
 <aside class="console-sidebar"><a class="console-brand" href="#" @click.prevent="page='overview'"><b>◆</b> {{siteTitle}}<small>VPS NODE CONTROL</small></a><button class="secondary console-home" :class="{active:page==='overview'}" @click="page='overview'">◉ 控制台概览</button><span class="nav-caption">节点管理工作流</span><nav aria-label="主管理导航"><button v-for="(p,i) in pages" :key="p.id" :class="{active:page===p.id}" @click="selectedUser='';page=p.id"><span class="step-number">0{{i+1}}</span><span>{{p.name}}<small>{{p.sub}}</small></span></button></nav><div class="sidebar-bottom"><button class="secondary" :class="{active:page==='settings'}" @click="page='settings'">⚙ 系统设置</button><p>节点 → 协议 → 权限 → 策略 → 订阅</p><small>nodehub · 统一节点控制面</small></div></aside>
 <main class="console-main"><header class="console-header"><span>工作区 <b>/ {{current.name}}</b></span><div><label class="theme-picker">主题<select v-model="theme" aria-label="主题"><option value="dark">深色</option><option value="light">浅色</option><option value="system">跟随系统</option></select></label><button class="text-button" @click="logout">退出登录</button></div></header>
 <div class="console-content"><div class="console-title"><div><span class="eyebrow">NODEHUB / {{page.toUpperCase()}}</span><h1>{{current.name}}</h1><p>{{current.sub}}</p></div><span class="control-domain">{{hostname}}</span></div><p v-if="error" class="error" role="alert">{{error}}</p>
 <Traffic v-if="page==='overview'||page==='usage'" :key="page+selectedUser" :api="api" :user-id="page==='usage'?selectedUser:undefined"/>
 <Hosts v-else-if="page==='hosts'" :api="api" :hosts="hosts" @changed="refresh" @configure="configure"/>
 <Topology v-else-if="page==='nodes'" :api="api" :hosts="hosts" :initial-host="selectedHost" @changed="refresh"/>
 <Users v-else-if="page==='users'" :api="api" @share="share" @usage="usage"/>
 <Policies v-else-if="page==='policies'" :api="api"/>
 <Subscriptions v-else-if="page==='subscriptions'" :api="api" :initial-user="selectedUser" @policies="page='policies'"/>
 <Settings v-else :api="api" :hosts="hosts" @branding="branding"/>
 </div><footer class="console-footer">{{siteTitle}} <span>连接状态与配置结果以 Agent 回报为准</span></footer>
 </main></div>
</div>
</template>
