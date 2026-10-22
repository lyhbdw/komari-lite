import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { build } from 'esbuild';
import { readFile, mkdtemp, rm } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
const file=resolve('src/pages/admin/index.tsx');
const source=await readFile(file,'utf8');
const imports=new Map();
const ast=ts.createSourceFile(file,source,ts.ScriptTarget.Latest,true,ts.ScriptKind.TSX);
for(const node of ast.statements){
 if(!ts.isImportDeclaration(node)||!node.importClause)continue;
 const path=node.moduleSpecifier.text;
 const names=[];const clause=node.importClause;
 if(clause.namedBindings&&ts.isNamedImports(clause.namedBindings))for(const item of clause.namedBindings.elements)names.push(item.propertyName?.text??item.name.text);
 imports.set(path,{names,hasDefault:!!clause.name,namespace:clause.namedBindings&&ts.isNamespaceImport(clause.namedBindings)});
}
let state,notices,refreshes,index;
const noop=()=>{};
const generic=new Proxy(noop,{get:()=>generic});
const nodes=[{uuid:'ok',name:'one',weight:0,group:''},{uuid:'failed',name:'two',weight:1,group:''}];
globalThis.__batchPage={
 generic,
 react:{createElement:(type,props,...children)=>({type,props:{...props,children}}),
  useState:initial=>{const slot=index++;if(!(slot in state))state[slot]=slot===3?['ok','failed']:typeof initial==='function'?initial():initial;return [state[slot],value=>{state[slot]=typeof value==='function'?value(state[slot]):value}]},
  useEffect:noop,useMemo:fn=>fn(),useCallback:fn=>fn},
 hooks:{useNodeDetails:()=>({nodeDetail:nodes,isLoading:false,error:null,refresh:()=>{refreshes++}}),useAccount:()=>({account:{logged_in:true}}),useSettings:()=>({settings:{}})},
 toast:{success:message=>notices.push(['success',message]),error:message=>notices.push(['error',message])},
};
const dir=await mkdtemp(resolve('tests/.tmp-'));
let Layout;
try{
 const outfile=resolve(dir,'page.mjs');
 await build({entryPoints:[file],outfile,bundle:true,platform:'node',format:'esm',jsx:'transform',alias:{'@':resolve('src')},plugins:[{name:'page-dependencies',setup(builder){
  builder.onResolve({filter:/.*/},args=>imports.has(args.path)&&args.path!=='@/utils/batchNodes'&&!args.path.includes('groupCore')&&!args.path.includes('billingCore')?{path:args.path,namespace:'stub'}:undefined);
  builder.onLoad({filter:/.*/,namespace:'stub'},args=>{
   const {names,hasDefault,namespace}=imports.get(args.path);let code='const s=globalThis.__batchPage;\n';
   if(args.path==='react')code+='export default s.react;export const {useState,useEffect}=s.react;';
   else if(args.path==='sonner')code+='export const toast=s.toast;';
   else {
    if(hasDefault)code+='export default s.generic;';
    for(const name of names)code+=`export const ${name}=s.hooks[${JSON.stringify(name)}]??s.generic;`;
    if(namespace){
     const alias=ast.statements.find(node=>ts.isImportDeclaration(node)&&node.moduleSpecifier.text===args.path).importClause.namedBindings.name.text;
     const members=new Set();
     function visit(node){if(ts.isPropertyAccessExpression(node)&&ts.isIdentifier(node.expression)&&node.expression.text===alias)members.add(node.name.text);ts.forEachChild(node,visit);}
     visit(ast);
     for(const member of members)code+=`export const ${member}=s.generic;`;
    }
   }
   return {contents:code,loader:'js'};
  });
  builder.onLoad({filter:/pages\/admin\/index\.tsx$/},()=>({contents:source+'\nexport {Layout};',loader:'tsx'}));
 }}]});
 ({Layout}=await import(pathToFileURL(outfile).href));
}finally{await rm(dir,{recursive:true,force:true});}
function find(tree,keyword){
 if(!tree||typeof tree!=='object')return;
 if(tree.type?.name===keyword)return tree;
 for(const child of [tree.props?.children].flat(2)){const result=find(child,keyword);if(result)return result;}
}
for(const [name,dialog] of [['group','BatchGroupDialog'],['delete','BatchDeleteDialog']])test(`${name} actual page handler shows partial failure and retains failed selection`,async()=>{
 state=[];notices=[];refreshes=0;index=0;
 const saved=globalThis.fetch;
 globalThis.fetch=async url=>url.includes('/failed/')?Response.json({status:'error',message:'denied'}):Response.json({status:'success'});
 try{
  const tree=Layout();const handler=find(tree,dialog)?.props.onConfirm;
  assert.equal(typeof handler,'function');
  await handler('new');
  assert.deepEqual(state[3],['failed']);assert.equal(refreshes,1);
  assert.equal(notices.length,1);assert.equal(notices[0][0],'error');assert.match(notices[0][1],/成功 1 台，失败 1 台/);
 }finally{globalThis.fetch=saved;}
});
