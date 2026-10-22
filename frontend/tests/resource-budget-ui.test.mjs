import assert from 'node:assert/strict';
import test from 'node:test';
import { build } from 'esbuild';
import { mkdtemp, rm } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
const dir=await mkdtemp(resolve('tests/.tmp-'));
let ResourceBudgetPanels;
try {
 const outfile=resolve(dir,'panel.mjs');
 await build({entryPoints:['src/components/admin/ResourceBudget.tsx'],outfile,bundle:true,platform:'node',format:'esm',packages:'external',jsx:'automatic',alias:{'@':resolve('src')}});
 ({ResourceBudgetPanels}=await import(pathToFileURL(outfile).href));
}finally{await rm(dir,{recursive:true,force:true});}
test('read-only panels render configured/effective/source and distinguish unavailable store',()=>{
 const html=renderToStaticMarkup(createElement(ResourceBudgetPanels,{
 retention:{raw_retention_seconds:600,metrics:[{metric_name:'ping.loss',definition_retention_days:90,enabled:true,configured_tiers:[{interval_seconds:86400,retention_seconds:2592000,source:'store_policy'}],effective_tiers:[{interval_seconds:86400,retention_seconds:7776000,source:'metric_definition'}]}]},
 memory:{store_ready:false,go:{},history_query_slots:{capacity:2,in_use:0}},
 }));
 for(const text of ['保留策略','配置','生效','来源','30 天','90 天','指标定义优先','内存预算','指标存储未初始化','不是进程 RSS'])assert.ok(html.includes(text),text);
 assert.equal(/<input|<select|<button/.test(html),false);
});
test('RPC errors are visible instead of zero budgets',()=>{
 const html=renderToStaticMarkup(createElement(ResourceBudgetPanels,{retentionError:'Permission denied',memoryError:'Unavailable'}));
 assert.match(html,/Permission denied/);assert.match(html,/Unavailable/);
 assert.equal(html.includes('0 B'),false);
});
