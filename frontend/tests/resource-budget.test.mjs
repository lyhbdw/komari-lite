import assert from 'node:assert/strict';
import test from 'node:test';
import { build } from 'esbuild';
import { mkdtemp, rm } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
const dir=await mkdtemp(resolve('tests/.tmp-'));
let formatBytes,formatDuration,formatRatio,queueTone,retentionRows,memoryRows;
try {
 const outfile=resolve(dir,'module.mjs');
 await build({entryPoints:['src/utils/resourceBudget.ts'],outfile,bundle:true,platform:'node',format:'esm'});
 ({formatBytes,formatDuration,formatRatio,queueTone,retentionRows,memoryRows}=await import(pathToFileURL(outfile).href));
}finally{await rm(dir,{recursive:true,force:true});}
test('budget units, unknown/disabled limit and queue warning thresholds',()=>{
 assert.equal(formatBytes(1048576),'1 MiB');
 assert.equal(formatBytes(-1),'—');
 assert.equal(formatBytes(undefined),'—');
 assert.equal(formatRatio(80,100),'80.0%');
 assert.equal(formatRatio(0,0),'未设置');
 assert.equal(formatRatio(1,9223372036854776000),'未设置');
 assert.equal(queueTone(8,10),'warning');
 assert.equal(queueTone(10,10),'danger');
 assert.equal(queueTone(0,0),'normal');
 assert.equal(formatDuration(7776000),'90 天');
 assert.equal(formatDuration(10800),'3 小时');
});
test('retention rows preserve configured 30d versus effective definition 90d and pruned tiers',()=>{
 const rows=retentionRows({metric_name:'ping.loss',enabled:true,definition_retention_days:90,configured_tiers:[{interval_seconds:86400,retention_seconds:2592000,source:'store_policy'}],effective_tiers:[{interval_seconds:86400,retention_seconds:7776000,source:'metric_definition'}]});
 assert.deepEqual(rows,[{interval:'1 天',configured:'30 天',effective:'90 天',source:'指标定义优先'}]);
 assert.equal(retentionRows({enabled:false,configured_tiers:[{interval_seconds:60,retention_seconds:10800}],effective_tiers:[]})[0].effective,'停用（不会主动清理历史）');
});
test('memory rows label structure estimates and never pretend uninitialized store is zero',()=>{
 const base={store_ready:false,go:{heap_alloc_bytes:1048576,soft_memory_limit_bytes:2097152},metric:{estimated_raw_bytes:0},queues:{},pools:{},history_query_slots:{capacity:2,in_use:1}};
 const rows=memoryRows(base);
 assert.equal(rows.find(r=>r.label==='Go HeapAlloc').value,'1 MiB · 50.0% 软预算');
 assert.equal(rows.find(r=>r.label==='指标存储').value,'指标存储未初始化');
 assert.equal(rows.some(r=>r.label.includes('结构估算')),false);
 const ready=memoryRows({...base,store_ready:true,metric:{estimated_raw_bytes:1024,estimated_hot_bytes:2048,estimated_coarse_bytes:4096},queues:{reports_queued:8,reports_capacity:10,pings_queued:1,pings_capacity:10},pools:{wait_count:3},history_query_slots:{in_use:2,capacity:2}});
 assert.equal(ready.find(r=>r.label==='Raw 结构估算').value,'1 KiB');
 assert.equal(ready.find(r=>r.label==='报告队列').tone,'warning');
 assert.equal(ready.find(r=>r.label==='历史查询槽').value,'2 / 2');
 assert.equal(ready.find(r=>r.label==='数据库等待次数').value,'3');
});
