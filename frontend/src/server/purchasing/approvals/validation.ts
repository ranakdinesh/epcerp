import { invalid,object,string,trim,version } from '../core/validation';
import type { Nomination,Command,Decision } from './types';
const pattern=/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
export const quoteKey=/^[0-9a-f-]{36}:[1-9][0-9]{0,9}$/;
export function id(raw:unknown){if(typeof raw!=='string'||!pattern.test(raw))invalid('id','Use a valid record identifier');return raw;}
export function nominations(raw:unknown):Nomination[]{
    if(!Array.isArray(raw)||!raw.length||raw.length>2)invalid('proposals','Choose one best-price and one preferred offer');
    let best=0,preferred=0;const seen=new Set<string>(),rows:Nomination[]=[];
    for(const value of raw){const n=object(value,['quote_key','best_price','preferred','best_price_comment','preferred_comment']);for(const key of ['best_price','preferred'])if(n[key]!==undefined&&typeof n[key]!=='boolean')invalid('proposals','Use true or false for designations');const row:Nomination={quote_key:string(n.quote_key,'quote_key',''),best_price:n.best_price===true,preferred:n.preferred===true,best_price_comment:trim(string(n.best_price_comment,'best_price_comment','')),preferred_comment:trim(string(n.preferred_comment,'preferred_comment',''))};
        if(!quoteKey.test(row.quote_key)||seen.has(row.quote_key)||!row.best_price&&!row.preferred)invalid('proposals','Choose distinct saved quotation revisions with a designation');seen.add(row.quote_key);if(row.best_price){best++;if(!row.best_price_comment)invalid('best_price_comment','Explain the best-price choice');}if(row.preferred){preferred++;if(!row.preferred_comment)invalid('preferred_comment','Explain the preferred choice');}if([...row.best_price_comment].length>2000||[...row.preferred_comment].length>2000)invalid('proposals','Each designation comment must be at most 2000 characters');rows.push(row);
    }
    if(best!==1||preferred!==1)invalid('proposals','Choose exactly one best-price and one preferred offer; one offer may have both');return rows;
}
export function command(raw:Record<string,unknown>,action:string):Command{
    const b=object(raw,['proposals','selected_quote_key','return_action','version','reason','quote_key','attempt_id','decisions']);const explanation=trim(string(b.reason,'reason',''));
    if(!explanation||[...explanation].length>2000||!Number.isInteger(b.version??0)||Number(b.version??0)<0)invalid('reason','Provide the saved version and a reason of at most 2000 characters');
    const selected_quote_key=string(b.selected_quote_key,'selected_quote_key',''),return_action=string(b.return_action,'return_action',''),key=string(b.quote_key,'quote_key',''),attempt_id=string(b.attempt_id,'attempt_id','');
    if(b.decisions!=null&&!Array.isArray(b.decisions))invalid('decisions','Review every item in the current submission');if(b.proposals!=null&&!Array.isArray(b.proposals))invalid('proposals','Submit only saved shortlist nominations');const decisions:Decision[]=[],rawDecisions=b.decisions as unknown[]??[],rawProposals=b.proposals as unknown[]??[];
    if(action==='submit'){
        if(rawProposals.length){if(key||attempt_id||rawDecisions.length||selected_quote_key||return_action)invalid('proposals','Submit only saved shortlist nominations');return{proposals:nominations(rawProposals),selected_quote_key,return_action,version:version(b.version??0),reason:explanation,quote_key:key,attempt_id,decisions};}
        if(selected_quote_key||return_action)invalid('quote_key','Review fields cannot be submitted');if(!quoteKey.test(key)||key.length>60||!pattern.test(key.split(':')[0])||attempt_id||rawDecisions.length)invalid('quote_key','Choose a saved quotation revision only');
    }else if(action==='review'){
        if(rawProposals.length||selected_quote_key&&!quoteKey.test(selected_quote_key)||return_action&&!['negotiate','change_vendor'].includes(return_action))invalid('return_action','Choose negotiate or change vendor when rejecting');if(!pattern.test(attempt_id)||key||rawDecisions.length<1||rawDecisions.length>100)invalid('decisions','Review every item in the current submission');
        for(const raw of rawDecisions){const d=object(raw,['line_id','decision','comment']),line_id=string(d.line_id,'line_id',''),decision=string(d.decision,'decision',''),comment=trim(string(d.comment,'comment',''));if(!pattern.test(line_id)||!['accept','reject'].includes(decision)||[...comment].length>2000)invalid('decisions','Each item needs accept/reject; optional notes must be at most 2000 characters');decisions.push({line_id,decision,comment});}
    }else invalid('action','Choose submit or review');
    return{proposals:[],selected_quote_key,return_action,version:version(b.version??0),reason:explanation,quote_key:key,attempt_id,decisions};
}
