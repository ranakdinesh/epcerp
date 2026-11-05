import { timestamp } from '../../inbox/repository';
import { invalid,isoDate } from './validation';
export const now=()=>timestamp(new Date().toISOString())!;
export function instant(value:unknown,field:string,utc=false){
    if(typeof value!=='string')invalid(field,'Use a valid timestamp');
    const match=/^(\d{4}-\d\d-\d\d)T(\d\d):(\d\d):(\d\d)(\.\d{1,9})?(Z|[+-]\d\d:\d\d)$/.exec(value);
    if(!match||!isoDate(match[1])||Number(match[2])>23||Number(match[3])>59||Number(match[4])>59||!Number.isFinite(Date.parse(value)))invalid(field,'Use a valid timestamp');
    const fraction=match[5]??'';
    return timestamp(utc?new Date(value).toISOString().slice(0,19)+fraction+'Z':value.replace(/\+00:00$/,'Z'))!;
}
export function indiaInstant(value=now()){return timestamp(new Date(Date.parse(value)+19800000).toISOString())!.replace(/Z$/,'+05:30');}
