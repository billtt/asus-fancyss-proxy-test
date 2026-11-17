'use strict';
const $ = id => document.getElementById(id);
const label = {PROXY:'预期代理',DIRECT:'预期直连',UNKNOWN:'无法判断'};
$('query').addEventListener('submit', async event => {
 event.preventDefault(); if ($('submit').disabled) return;
 const domain=$('domain').value.trim(); $('submit').disabled=true;$('submit').textContent='查询中…';$('results').setAttribute('aria-busy','true');
 $('status').textContent='正在解析 DNS 并核对规则…';$('rows').replaceChildren();$('table').hidden=true;$('warning').hidden=true;$('meta').textContent='';
 const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),23000);
 try {
  const response=await fetch('/api/query',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({domain}),signal:controller.signal,cache:'no-store'});
  if(!response.ok) throw new Error((await response.text()).trim());
  const data=await response.json();
  const counts={PROXY:0,DIRECT:0,UNKNOWN:0};data.rows.forEach(row=>{
   counts[row.decision]++;const tr=document.createElement('tr');
   const hits=Object.entries(row.sets).filter(([,state])=>state==='HIT').map(([set])=>set);
   const errors=Object.entries(row.sets).filter(([,state])=>state==='ERROR').map(([set])=>set+'（检测失败）');
   [row.ip,label[row.decision],row.rule,[...hits,...errors].join(' · ')||'无命中'].forEach((text,i)=>{const td=document.createElement('td');td.textContent=text;if(i===1)td.className=row.decision.toLowerCase();tr.append(td)});$('rows').append(tr);
  });
  $('status').textContent=`${data.rows.length} 个 IPv4 · ${counts.PROXY} 代理 / ${counts.DIRECT} 直连 / ${counts.UNKNOWN} 无法判断`;
  $('meta').textContent=`${data.domain} · DNS ${data.dns} · ${new Date(data.time).toLocaleString()} · ${data.verified?'规则顺序已核对':'规则未通过验证'}`;
  $('warning').textContent=data.warning;$('warning').hidden=!data.warning;$('table').hidden=false;
 } catch(error){$('status').textContent=error.name==='AbortError'?'查询超时，请稍后重试。':'查询未完成：'+error.message;}
 finally{clearTimeout(timer);$('submit').disabled=false;$('submit').textContent='查询分流 ↗';$('results').setAttribute('aria-busy','false');}
});
document.querySelectorAll('[data-domain]').forEach(button=>button.addEventListener('click',()=>{$('domain').value=button.dataset.domain;$('query').requestSubmit()}));
