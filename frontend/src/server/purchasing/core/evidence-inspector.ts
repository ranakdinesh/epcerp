import sharp from 'sharp';
import { Inflate } from 'fflate';
import { invalid } from './validation';
const mimeXLSX='application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
const bad=()=>invalid('file','Upload a valid PDF, PNG, JPEG or XLSX quotation file matching its filename and content type');
const crcTable=Array.from({length:256},(_,n)=>{let c=n;for(let k=0;k<8;k++)c=c&1?0xedb88320^(c>>>1):c>>>1;return c>>>0;});
function crc32(data:Uint8Array,crc:number){for(const byte of data)crc=crcTable[(crc^byte)&255]^(crc>>>8);return crc;}
function workbook(data:Buffer){
    let end=-1;for(let i=data.length-22;i>=Math.max(0,data.length-65557);i--)if(data.readUInt32LE(i)===0x06054b50){end=i;break;}
    if(end<0||end+22+data.readUInt16LE(end+20)!==data.length||data.readUInt16LE(end+4)!==0||data.readUInt16LE(end+6)!==0)bad();
    const count=data.readUInt16LE(end+10),offset=data.readUInt32LE(end+16);if(count>2000||data.readUInt16LE(end+8)!==count||offset+data.readUInt32LE(end+12)>end)bad();
    let cursor=offset,expanded=0;const entries=new Map<string,{size:number;crc:number;compressed:number;offset:number;method:number;flags:number}>();
    for(let i=0;i<count;i++){
        if(cursor+46>end||data.readUInt32LE(cursor)!==0x02014b50)bad();const size=data.readUInt32LE(cursor+24),length=data.readUInt16LE(cursor+28),extra=data.readUInt16LE(cursor+30),comment=data.readUInt16LE(cursor+32),next=cursor+46+length+extra+comment;if(next>end)bad();const name=data.subarray(cursor+46,cursor+46+length).toString('utf8');
        if(entries.has(name)||name.includes('\\')||name.startsWith('/')||('/'+name+'/').includes('/../')||name.toLowerCase().includes('vbaproject')||size>32*1024*1024||expanded>64*1024*1024-size)bad();expanded+=size;entries.set(name,{size,crc:data.readUInt32LE(cursor+16),compressed:data.readUInt32LE(cursor+20),offset:data.readUInt32LE(cursor+42),method:data.readUInt16LE(cursor+10),flags:data.readUInt16LE(cursor+8)});cursor=next;
    }
    if(!entries.has('[Content_Types].xml')||!entries.has('xl/workbook.xml'))bad();
    for(const entry of entries.values()){
        const local=entry.offset;if(local+30>offset||data.readUInt32LE(local)!==0x04034b50||entry.flags&1)bad();
        const start=local+30+data.readUInt16LE(local+26)+data.readUInt16LE(local+28),end=start+entry.compressed;
        if(end>offset||start>end||![0,8].includes(entry.method))bad();let count=0,crc=0xffffffff;
        const consume=(chunk:Uint8Array)=>{count+=chunk.length;if(count>entry.size)bad();crc=crc32(chunk,crc);};
        if(entry.method===0)consume(data.subarray(start,end));
        else{try{const stream=new Inflate(chunk=>consume(chunk));if(start===end)stream.push(new Uint8Array(),true);for(let at=start;at<end;at+=1024)stream.push(data.subarray(at,Math.min(end,at+1024)),at+1024>=end);}catch{return bad();}}
        if(count!==entry.size||((crc^0xffffffff)>>>0)!==entry.crc)bad();
    }
}
export async function inspectQuotation(name:string,mime:string,data:Buffer){
    if(!data.length||data.length>10*1024*1024||!name||[...name].length>255||/[\/\\\x00-\x1f\x7f]/.test(name)||/[\uD800-\uDFFF]/u.test(name))bad();
    const ext=name.slice(name.lastIndexOf('.')).toLowerCase();
    if(ext==='.pdf'){if(mime!=='application/pdf'||data.subarray(0,5).toString()!=='%PDF-'||!data.subarray(-2048).includes(Buffer.from('%%EOF')))bad();return;}
    if(ext==='.xlsx'){if(mime!==mimeXLSX)bad();workbook(data);return;}
    if(ext==='.png'){if(mime!=='image/png'||!data.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])))bad();}
    else if(ext==='.jpg'||ext==='.jpeg'){if(mime!=='image/jpeg'||!data.subarray(0,3).equals(Buffer.from([255,216,255]))||!data.subarray(-2).equals(Buffer.from([255,217])))bad();}
    else bad();
    try{const meta=await sharp(data,{limitInputPixels:40000000}).metadata();if(!meta.width||!meta.height||meta.width*meta.height>40000000||(mime==='image/png'&&meta.format!=='png')||(mime==='image/jpeg'&&meta.format!=='jpeg'))bad();}catch{return bad();}
}
export async function inspectTechnical(name:string,mime:string,data:Buffer){
    try{if(!['application/pdf','image/png','image/jpeg'].includes(mime))throw new Error('Unsupported type');await inspectQuotation(name,mime,data);if(mime!=='application/pdf')await sharp(data,{limitInputPixels:40000000}).raw().toBuffer();}
    catch{invalid('file','Upload a valid PDF, PNG or JPEG matching its filename and content type');}
}
