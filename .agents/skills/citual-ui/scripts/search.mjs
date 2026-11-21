#!/usr/bin/env node
import { readFile } from 'node:fs/promises';
import { realpathSync } from 'node:fs';
import { parseArgs } from 'node:util';
import { pathToFileURL } from 'node:url';
import path from 'node:path';
export function searchCatalog(catalog, query = '', options = {}) {
 const words = text => text.replace(/([a-z])([A-Z])/g,'$1 $2').toLowerCase().split(/[^a-z0-9]+/).filter(Boolean).map(w => w.length>4 && w.endsWith('s') ? w.slice(0,-1) : w);
 const stop = new Set(['a','an','the','i','we','need','want','to','for','and','with','my','our','of','in','build','please','component','ui']);
 const terms = [...new Set(words(query).filter(w => !stop.has(w)))];
 const fields = [['id',10],['export',10],['description',5],['useWhen',4],['keywords',6],['capabilities',5],['domains',2]];
 return catalog.items.filter(item => (!options.kind || item.kind===options.kind) && (!options.maturity || item.maturity===options.maturity) && (!options.generic || item.generic) && (!options.capabilities || options.capabilities.every(c => item.capabilities.includes(c)))).map(item => {
  const matched=[]; let score=0;
  for (const term of terms) {
   let best=0;
   for (const [key,weight] of fields) if (words(Array.isArray(item[key]) ? item[key].join(' ') : String(item[key]??'')).some(w=>w===term)) best=Math.max(best,weight);
   if(best) {score+=best;matched.push(term);}
  }
  if (terms.length && !matched.length) return null;
  // Coverage rewards complete matches; generic workflows win ties, not irrelevant queries.
  score += terms.length ? matched.length/terms.length*10 : 0;
  if(item.generic) score+=1;
  return {...item, score, matchedTerms:matched, reason:terms.length ? `Matched ${matched.join(', ')}; ${item.useWhen}` : item.useWhen};
 }).filter(Boolean).sort((a,b)=>b.score-a.score || a.id.localeCompare(b.id)).slice(0,options.limit??5);
}
export async function main(args=process.argv.slice(2)) {
 const {values,positionals}=parseArgs({args,allowPositionals:true,options:{q:{type:'string'},id:{type:'string'},catalog:{type:'string'},format:{type:'string',default:'json'},limit:{type:'string',default:'5'},kind:{type:'string'},maturity:{type:'string'},generic:{type:'boolean',default:false},capability:{type:'string',multiple:true}}});
 if(!['json','text'].includes(values.format)) throw new Error('format must be json or text');
 if(!/^\d+$/.test(values.limit) || +values.limit<1 || +values.limit>100) throw new Error('limit must be 1–100');
 if(values.kind && !['workflow','component','layout'].includes(values.kind)) throw new Error('kind must be workflow, component or layout');
 const catalogPath=values.catalog ? pathToFileURL(path.resolve(values.catalog)) : new URL('../references/catalog.json',import.meta.url);
 const catalog=JSON.parse(await readFile(catalogPath,'utf8'));
 const command=positionals[0]??'search';
 if(!['search','show'].includes(command)) throw new Error('Use search --q "intent" or show --id component-id');
 const found=command==='show' ? catalog.items.filter(item=>item.id===values.id) : searchCatalog(catalog,values.q??positionals.slice(1).join(' '),{kind:values.kind,maturity:values.maturity,generic:values.generic,capabilities:values.capability,limit:+values.limit});
 const results=command==='show' ? found : found.map(({types,...item})=>({...item,typeNames:types.map(type=>type.name)}));
 console.log(values.format==='json' ? JSON.stringify({schemaVersion:1,query:values.q??'',results},null,2) : results.map(item=>`${item.export} [${item.id}; ${item.maturity}]\n${item.description}\nUse: ${item.useWhen}\nAvoid: ${item.avoidWhen}\n${item.import}\nSource: ${item.path}\nRecipe: ${item.recipe}\nBackend: ${item.backend}`).join('\n\n') || 'No matches. Broaden the query or remove capability filters.');
 if(command==='show' && !found.length) process.exitCode=1;
}
if (process.argv[1] && import.meta.url===pathToFileURL(realpathSync(process.argv[1])).href) main().catch(error=>{console.error(error.message);process.exitCode=1;});
