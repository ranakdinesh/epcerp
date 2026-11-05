import { amount,Rational } from '../core/rational';
import { format } from '../core/decimal';
import { invalid,isoDate,trim } from '../core/validation';
import type { QuoteInput,Document,Calculation,CalcLine,Quote,Comparison } from './types';
const zero=BigInt(0),one=BigInt(1),hundred=Rational.integer(100);
export function calculate(q:QuoteInput,d:Document,today:string):Calculation{
    const out:Calculation={lines:[],total:'',complete:false,compliant:true};
    if(!d.data.sent_date||!isoDate(d.data.sent_date)||q.received_date<d.data.sent_date||q.received_date>today)invalid('received_date','Received date must be between indent dispatch and today');if(q.valid_until<q.received_date)invalid('valid_until','Quote validity cannot precede receipt');
    const mapped=new Map(q.lines.map(l=>[l.line_id,l]));if(mapped.size!==q.lines.length||mapped.size!==d.data.lines.length)invalid('lines','Record every vendor-indent item exactly once');
    const row=(line:QuoteInput['lines'][number],source:Document['data']['lines'][number]):CalcLine=>({...line,quantity:source.quantity!,source,base:null,discount_amount:null,net:null,tax_amount:null,landed:null,allocated_charges:null,item_lowest:false});
    if(q.amount_mode==='total_only'){
        if(q.total_amount===null)invalid('total_amount',"Enter the vendor's overall quotation total");const total=amount(q.total_amount,20,2);if(total.numerator<=zero)invalid('total_amount','Overall quotation total must be a positive amount');
        for(const source of d.data.lines){const line=mapped.get(source.id);if(!line)invalid('lines','Record every vendor-indent item exactly once');if(source.quantity===null)invalid('quantity','Indent quantity is missing');if(amount(source.quantity,13,3).numerator<=zero)invalid('quantity','Indent quantities must be positive');out.lines.push(row(line,source));}
        return{...out,total:total.fixed(),complete:true,compliant:false};
    }
    const priced:number[]=[],weights:bigint[]=[];let sumWeights=zero,total=zero;
    for(const source of d.data.lines){
        const line=mapped.get(source.id);if(!line)invalid('lines','Record every vendor-indent item exactly once');if(source.quantity===null)invalid('quantity','Indent quantity is missing');const quantity=amount(source.quantity,13,3);if(quantity.numerator<=zero)invalid('quantity','Indent quantities must be positive');const r=row(line,source);out.compliant=out.compliant&&line.compliant;
        const mode=line.pricing_mode||'unit_price';if(!['unit_price','line_total'].includes(mode))invalid('pricing_mode','Choose unit price or line total');let price=line.unit_price;
        if(mode==='line_total'){if(line.unit_price!==null)invalid('unit_price','Enter only line total when using direct amount');price=line.line_total;}else if(line.line_total!==null)invalid('line_total','Enter only unit price when using unit rate');
        if(price===null){if(!trim(line.notes))invalid('notes','Record why an item is not quoted');out.lines.push(r);continue;}
        if(line.warranty_months===null||![6,12,18,24,36,48,60,84].includes(line.warranty_months))invalid('warranty_months','Select warranty/guarantee months for every quoted SKU');if(!['warranty','guarantee','both'].includes(line.coverage_type))invalid('coverage_type','Choose warranty, guarantee or both');
        const rate=amount(price,mode==='line_total'?18:12,mode==='line_total'?2:4);let base=quantity.multiply(rate).round(2);if(mode==='line_total'){base=rate.round(2);r.unit_price=Rational.cents(base).divide(quantity).fixed();}
        const discount=amount(line.discount_pct,3,2),tax=amount(line.tax_pct,3,2);if(discount.compare(hundred)>0)invalid('discount_pct','Discount must be from zero to 100');if(tax.compare(hundred)>0)invalid('tax_pct','Tax must be from zero to 100');
        const reduction=Rational.cents(base).multiply(discount).divide(hundred).round(2),net=base-reduction,taxAmount=Rational.cents(net).multiply(tax).divide(hundred).round(2),landed=net+taxAmount;
        r.base=format(base,2);r.discount_amount=format(reduction,2);r.net=format(net,2);r.tax_amount=format(taxAmount,2);r.landed=format(landed,2);priced.push(out.lines.length);weights.push(net);sumWeights+=net;total+=landed;out.lines.push(r);
    }
    if(!priced.length)invalid('lines','Record a price for at least one item');const extras=amount(q.freight,14,2).add(amount(q.other_charges,14,2)).round(2);
    if(sumWeights===zero){sumWeights=BigInt(weights.length);weights.fill(one);}
    const shares=weights.map(w=>extras*w/sumWeights),remainders=weights.map(w=>extras*w%sumWeights),order=weights.map((_,i)=>i).sort((a,b)=>remainders[a]===remainders[b]?0:remainders[a]>remainders[b]?-1:1),left=Number(extras-shares.reduce((a,b)=>a+b,zero));
    for(let i=0;i<left;i++)shares[order[i]]++;
    for(const[i,index]of priced.entries()){out.lines[index].allocated_charges=format(shares[i],2);out.lines[index].landed=format(amount(out.lines[index].landed!,64,2).round(2)+shares[i],2);}
    total+=extras;if(total>BigInt('9999999999999999999999'))invalid('total','Quotation total exceeds supported limit');return{...out,total:format(total,2),complete:priced.length===out.lines.length};
}
export function compare(quotes:Quote[],documents:Document[],today:string,currency:string):Comparison[]{
    const out:Comparison[]=[];
    for(const q of quotes){const d=documents.find(d=>d.id===q.document_id);if(!d)continue;const exclusions:string[]=[],expired=q.valid_until<today;
        if(!d.data.is_current)exclusions.push('Superseded indent scope — revised quotation required');if(q.currency!==currency)exclusions.push('Quotation currency differs from the project');if(q.amount_mode==='total_only')exclusions.push('Overall quotation total has no item allocation — record item prices before Accounts approval');if(!q.calculation.complete)exclusions.push('Partial quotation');if(!q.calculation.compliant)exclusions.push('Technical compliance not confirmed for every item');if(!d.vendor_available)exclusions.push('Vendor inactive or unavailable');if(expired)exclusions.push('Quotation expired');
        out.push({document_id:q.document_id,amount_mode:q.amount_mode,total_amount:q.total_amount,group_id:d.group_id,group_name:d.data.group_name,scope_revision:d.data.scope_revision,vendor_id:d.vendor_id,vendor:d.data.vendor.name,reference:q.reference,total:q.calculation.total,eligible:!exclusions.length,exclusions,expired,delivery_days:q.delivery_days,payment_terms:q.payment_terms,delivery_terms:q.delivery_terms,lines:structuredClone(q.calculation.lines),rank:null,suggested:false,revision:q.revision,received_date:q.received_date,valid_until:q.valid_until,recorded_at:q.recorded_at,reason:q.reason});
    }
    for(const row of out){if(!row.eligible)continue;const own=amount(row.total,20,2),lower=new Set(out.filter(other=>other.eligible&&other.group_id===row.group_id&&amount(other.total,20,2).compare(own)<0).map(other=>other.total));row.rank=lower.size+1;row.suggested=row.rank===1;
        for(const line of row.lines){if(line.landed===null)continue;const price=amount(line.landed,20,2);line.item_lowest=!out.some(other=>other.eligible&&other.group_id===row.group_id&&other.lines.some(candidate=>candidate.line_id===line.line_id&&candidate.landed!==null&&amount(candidate.landed,20,2).compare(price)<0));}
    }
    return out;
}
