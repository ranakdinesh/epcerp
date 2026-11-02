#!/usr/bin/env node
import {readFile} from 'node:fs/promises';import {parseArgs} from 'node:util';import {resolve} from 'node:path';import {pathToFileURL} from 'node:url';import {realpathSync} from 'node:fs';
import {searchCatalog} from './search.mjs';
const normalize=text=>text.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean).map(word=>word.length>4&&word.endsWith('s')?word.slice(0,-1):word);
export function planScreen(catalog,patterns,prompt){
 const words=new Set(normalize(prompt));
 const ranked=patterns.recipes.map(recipe=>({...recipe,matchedTerms:normalize(recipe.keywords.join(' ')).filter(word=>words.has(word))})).filter(recipe=>recipe.matchedTerms.length).sort((a,b)=>b.matchedTerms.length-a.matchedTerms.length||a.id.localeCompare(b.id));
 const compact=item=>item?{id:item.id,export:item.export,import:item.import,path:item.path,recipe:item.recipe,preview:item.preview,previewContext:'Library showcase route, not a route in the consuming application',maturity:item.maturity,useWhen:item.useWhen,avoidWhen:item.avoidWhen,backend:item.backend,license:item.license}:undefined;
 const primary=ranked[0];
 return {schemaVersion:1,libraryVersion:catalog.version,prompt,status:primary?'candidates-found':'no-screen-match',note:'A deterministic retrieval plan, not generated code. The implementing agent chooses the final composition after reading source contracts and the existing project.',screen:primary?{id:primary.id,title:primary.title,matchedTerms:primary.matchedTerms,constraint:primary.constraint,sections:primary.sections.map(section=>({title:section.title,candidates:section.componentIds.map(id=>compact(catalog.items.find(item=>item.id===id)))})),layoutCandidates:primary.layoutQueries.map(query=>({query,candidates:[...searchCatalog(catalog,query,{kind:'layout',limit:2}),...searchCatalog(catalog,query,{kind:'component',limit:2})].sort((a,b)=>b.score-a.score||a.id.localeCompare(b.id)).slice(0,2).map(compact)}))}:null,alternatives:ranked.slice(1,3).map(recipe=>({id:recipe.id,title:recipe.title,matchedTerms:recipe.matchedTerms})),behaviorCandidates:searchCatalog(catalog,prompt,{generic:true,limit:5}).map(compact),implementation:['Read the existing framework, routes, brand tokens and data layer.','Compare at most two visual candidates; inspect source/types before importing.','Choose the page shell, content hierarchy, component composition and responsive behavior.','Implement the requested screen with real package imports and caller-owned callbacks.','Verify type/build, primary interactions, empty/error/pending states and desktop/mobile rendering.'],fallback:primary?undefined:'Search smaller visual/behavior sections. If none fit, explain the gap and build the requested adapter without pretending it came from the library.'};
}
export async function main(args=process.argv.slice(2)){
 const {values}=parseArgs({args,options:{q:{type:'string'},catalog:{type:'string'}}});if(!values.q?.trim())throw new Error('plan requires --q "describe the screen"');
 const catalog=JSON.parse(await readFile(values.catalog?resolve(values.catalog):new URL('../references/catalog.json',import.meta.url),'utf8'));
 const patterns=JSON.parse(await readFile(new URL('../references/screens.json',import.meta.url),'utf8'));
 console.log(JSON.stringify(planScreen(catalog,patterns,values.q),null,2));
}
if(process.argv[1]&&import.meta.url===pathToFileURL(realpathSync(process.argv[1])).href)main().catch(error=>{console.error(error.message);process.exitCode=1;});
