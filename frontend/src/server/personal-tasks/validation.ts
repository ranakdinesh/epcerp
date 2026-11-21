import { AppError } from '../errors';
export const invalid = (message: string): never => { throw new AppError(422,message); };
export function query(params: URLSearchParams) {
    for (const k of params.keys()) if (!['page','page_size','search','role','state'].includes(k) || params.getAll(k).length !== 1) invalid('Query field not supported or repeated');
    const number = (k: string, fallback: number) => {
        const s=params.get(k);if(s===null)return fallback;
        if(!/^[+]?\d+$/.test(s)||!Number.isSafeInteger(Number(s))||Number(s)<1||Number(s)>2147483647)invalid('Use a positive page or size');return Number(s);
    };
    const page=number('page',1),size=number('page_size',20),search=(params.get('search')??'').replace(/^[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+|[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+$/g,''),role=params.get('role')||'all',state=params.get('state')||'all';
    if(page>100000||![20,50,100].includes(size)||[...search].length>120||/[\uD800-\uDFFF]/u.test(search))invalid('Use bounded page/search and 20/50/100 rows');
    if(!['all','doer','resolver'].includes(role)||!['all','not_started','preparing','blocked'].includes(state))invalid('Choose a supported role and coordination state');
    return {page,size,search,role,state};
}
export type Query=ReturnType<typeof query>;
export const indiaDate=(now: Date)=>new Date(now.getTime()+19800000).toISOString().slice(0,10);
export const overdue=(relation:string,planned:string,due:string,now:Date)=>!!(relation==='resolver'?due:planned)&&(relation==='resolver'?due:planned)<indiaDate(now);
