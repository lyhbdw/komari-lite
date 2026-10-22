import assert from 'node:assert/strict';
import test from 'node:test';
import { build } from 'esbuild';
import { mkdtemp, rm } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
const dir = await mkdtemp(resolve('tests/.tmp-'));
let runBatchNodes, remainingSelection;
try {
 const outfile=resolve(dir,'batch.mjs');
 await build({entryPoints:['src/utils/batchNodes.ts'],outfile,bundle:true,platform:'node',format:'esm'});
 ({runBatchNodes,remainingSelection}=await import(pathToFileURL(outfile).href));
} finally {await rm(dir,{recursive:true,force:true});}
for (const action of ['group','delete']) {
 test(`${action}: HTTP, business and network failures retain only failed selections`,async()=>{
  const calls=[];
  const fetcher=async(url,options)=>{
   calls.push([url,options]);
   if(url.includes('/network/')) throw new TypeError('offline');
   if(url.includes('/http/')) return new Response('{"message":"denied"}',{status:403});
   if(url.includes('/business/')) return Response.json({status:'error',error:'cannot change'});
   if(url.includes('/rpc/')) return Response.json({error:{code:-1,message:'business failed'}});
   return Response.json({status:'success'});
  };
  const result=await runBatchNodes(['ok','http','business','network','rpc'],action,'new',fetcher);
  assert.deepEqual(result.succeeded,['ok']);
  assert.deepEqual(result.failed.map(x=>x.uuid),['http','business','network','rpc']);
  assert.match(result.failed[0].message,/denied/);
  assert.match(result.failed[2].message,/offline/);
  assert.deepEqual(remainingSelection(['ok','http','business','network','rpc','later'],result),['http','business','network','rpc','later']);
  assert.equal(calls.length,5);
  assert.equal(calls[0][1].method,'POST');
  assert.match(calls[0][0],action==='group'?/\/edit$/:/\/remove$/);
  if(action==='group') assert.deepEqual(JSON.parse(calls[0][1].body),{group:'new'});
 });
}
test('204 is accepted but malformed success JSON fails closed',async()=>{
 const result=await runBatchNodes(['empty','invalid'],'delete','',async url=>url.includes('/empty/')?new Response(null,{status:204}):new Response('<html>login</html>'));
 assert.deepEqual(result.succeeded,['empty']);
 assert.deepEqual(result.failed.map(x=>x.uuid),['invalid']);
});
