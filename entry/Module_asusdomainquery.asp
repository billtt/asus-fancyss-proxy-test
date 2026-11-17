<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="referrer" content="no-referrer"><title>域名分流查询入口</title><style>body{font:16px/1.7 system-ui,sans-serif;background:#f4f7f4;color:#172f33;max-width:680px;margin:12vh auto;padding:24px}a{color:#007b70}small{color:#657674}</style></head>
<body><!-- asus-domain-query-entry-v1 --><h1>域名分流查询</h1><p id="status">正在打开 LAN 查询工具…</p><p><a id="open" rel="noopener noreferrer">点击打开查询工具</a></p><p><a href="/Module_Softcenter.asp">返回软件中心</a></p><small>此卡片仅是入口。移除入口不会停止查询服务。</small>
<script>
(function(){
 'use strict';
 // Use the firmware LAN address, never query parameters or the WAN hostname.
 var lan='<% nvram_get("lan_ipaddr"); %>';
 var parts=lan.split('.');
 var valid=parts.length===4&&parts.every(function(p){return /^(0|[1-9][0-9]{0,2})$/.test(p)&&Number(p)<=255;});
 valid=valid&&(parts[0]==='10'||parts[0]==='192'&&parts[1]==='168'||parts[0]==='172'&&Number(parts[1])>=16&&Number(parts[1])<=31);
 if(!valid){document.getElementById('status').textContent='无法读取有效 LAN 地址，请使用已保存的查询工具地址。';return;}
 var target='http://'+lan+':18085/';document.getElementById('open').href=target;
 document.getElementById('status').textContent='正在打开 '+target;
 window.location.replace(target);
})();
</script></body></html>
