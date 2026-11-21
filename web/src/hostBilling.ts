export const hostBytes=(v:unknown)=>{const n=Number(v??0);for(const [unit,size] of [['TB',1e12],['GB',1e9],['MB',1e6],['KB',1e3]] as const){if(n>=size)return (n/size).toFixed(2)+' '+unit}return n+' B'}
export const bytesPerGB=1e9
export const hostTrafficBytes=(v:unknown)=>`${(Number(v??0)/bytesPerGB).toFixed(2)} GB`
export const hostModes:Record<string,string>={both:'双向 RX + TX',rx:'仅入站 RX',tx:'仅出站 TX',max:'周期累计 RX / TX 取大'}
export function hostDraft(t:Record<string,any>={}){const quotaBytes=Number(t.quota_bytes??0);return {version:t.version??1,mode:t.mode??'both',reset_day:t.reset_day??new Date().getDate(),auto_reset:t.auto_reset??true,interfaces:[...(t.interfaces??[])],quota:quotaBytes/bytesPerGB,quotaBytes}}
export function hostPayload(d:Record<string,any>){const quota=Number(d.quota);return {version:d.version,quota_bytes:quota===d.quotaBytes/bytesPerGB?d.quotaBytes:Math.round(quota*bytesPerGB),display_unit:'GB',mode:d.mode,reset_day:Number(d.reset_day),auto_reset:!!d.auto_reset,interfaces:d.interfaces}}
