"""Bounded, offline source verification. No network except the private malware scanner."""
import sys,os,json,zipfile,io,stat,re,unicodedata,socket,struct,hashlib,base64,subprocess,xml.etree.ElementTree as ET
LEAF=32*1048576
class Invalid(Exception):pass
def bad(code):raise Invalid(code)
def note(**v):print(json.dumps(v,separators=(',',':')),flush=True)
def path(name):
 if not name or len(name.encode())>2048 or name.startswith('/') or any(c in name for c in '\\:') or any(unicodedata.category(c)in('Cc','Cf','Cs')for c in name):bad('unsafe_archive_path')
 parts=name.rstrip('/').split('/')
 if len(parts)>10:bad('archive_depth_limit')
 if any(not p or p in('.','..')or p!=p.strip()or p.rstrip('. ')!=p for p in parts):bad('unsafe_archive_path')
 return unicodedata.normalize('NFC',name.rstrip('/')).lower()
def scan(data):
 address=os.environ.get('EPCERP_CLAMD_SOCKET','tcp://clamav:3310')
 def connect():
  if address.startswith('tcp://'):
   host,port=address[6:].rsplit(':',1);return socket.create_connection((host,int(port)),timeout=5)
  s=socket.socket(socket.AF_UNIX);s.settimeout(5);s.connect(address);return s
 try:
  with connect()as s:
   s.settimeout(120);s.sendall(b'zINSTREAM\0')
   for at in range(0,len(data),65536):b=data[at:at+65536];s.sendall(struct.pack('!I',len(b))+b)
   s.sendall(b'\0'*4);reply=s.recv(4096).rstrip(b'\0\r\n')
   if reply.endswith(b' FOUND'):bad('malware_detected')
   if reply!=b'stream: OK':bad('dependency_unavailable')
  with connect()as s:s.sendall(b'zVERSION\0');version=s.recv(256).rstrip(b'\0\r\n').decode()
  if not version.startswith('ClamAV '):bad('dependency_unavailable')
  return version
 except Invalid:raise
 except Exception:bad('dependency_unavailable')
def entries(data,max_count,max_expansion):
 try:z=zipfile.ZipFile(io.BytesIO(data))
 except Exception:bad('invalid_archive')
 items=z.infolist()
 if not items or len(items)>max_count:bad('member_limit')
 seen=set();expanded=0
 for f in items:
  n=path(f.filename)
  if n in seen:bad('archive_path_collision')
  seen.add(n);mode=f.external_attr>>16
  if f.flag_bits&1 or f.compress_type not in(0,8)or stat.S_IFMT(mode)not in(0,stat.S_IFREG,stat.S_IFDIR):bad('unsafe_archive_entry')
  if not f.is_dir():
   if f.file_size<1 or f.file_size>max_expansion or f.compress_size<1 or f.file_size>f.compress_size*100:bad('archive_expansion_limit')
   expanded+=f.file_size
   if expanded>max_expansion:bad('archive_expansion_limit')
 return z,items,expanded
def office(data,kind):
 z,items,expanded=entries(data,10000,64*1048576);found=False;sheets=cells=0
 for f in items:
  name=f.filename.lower()
  if any(v in name for v in ['vbaproject','activex','embeddings/']):bad('active_office_content')
  if f.is_dir():continue
  if f.filename==('word/document.xml'if kind=='docx'else'xl/workbook.xml'):found=True
  sheet=kind=='xlsx'and name.startswith('xl/worksheets/')and name.endswith('.xml');sheets+=bool(sheet)
  try:
   with z.open(f)as source:part=source.read(64*1048576+1)
   if len(part)!=f.file_size:bad('archive_integrity_failed')
   if name.endswith(('.xml','.rels')):
    if re.search(br'<!DOCTYPE|<!ENTITY',part,re.I):bad('active_office_content')
    depth=tokens=0
    for event,node in ET.iterparse(io.BytesIO(part),events=('start','end')):
     tokens+=1
     if tokens>1000000:bad('office_expansion_limit')
     if event=='start':
      depth+=1
      if sheet and node.tag.rsplit('}',1)[-1]=='c':cells+=1
      if any(k.rsplit('}',1)[-1]=='TargetMode'and v.lower()=='external'for k,v in node.attrib.items()):bad('external_office_reference')
     else:depth-=1;node.clear()
     if depth>100 or cells>100000 or sheets>100:bad('office_expansion_limit')
  except Invalid:raise
  except Exception:bad('invalid_office')
 if not found:bad('invalid_office')
 return max(expanded,len(data)),sheets,cells
def run():
 name=sys.argv[2];data=open(sys.argv[1],'rb').read(64*1048576+1)
 if len(data)>64*1048576:bad('payload_too_large')
 scan(data);members=pages_total=sheets_total=cells_total=expanded_total=0;seen=set()
 def leaf(name,data):
  nonlocal members,pages_total,sheets_total,cells_total,expanded_total
  n=path(name)
  if n in seen:bad('archive_path_collision')
  seen.add(n);members+=1
  if members>100:bad('member_limit')
  if not data or len(data)>LEAF:bad('payload_too_large')
  kind=name.rsplit('.',1)[-1].lower()
  if kind not in('pdf','docx','xlsx'):bad('unsupported_media')
  revision=scan(data);pages=sheets=cells=0;expanded=len(data)
  if kind=='pdf':
   if not data.startswith(b'%PDF-'):bad('unsupported_media')
   source=os.path.join(os.path.dirname(sys.argv[1]),'leaf.pdf');open(source,'wb').write(data)
   try:p=subprocess.run(['/usr/bin/prlimit','--as=536870912','--cpu=30','--nofile=64','--','/usr/bin/pdfinfo',source],capture_output=True,timeout=35)
   except Exception:bad('invalid_pdf')
   if p.returncode or len(p.stdout)>65536 or not re.search(br'^Encrypted:\s+no\s*$',p.stdout,re.M):bad('invalid_pdf')
   m=re.search(br'^Pages:\s+(\d+)\s*$',p.stdout,re.M);pages=int(m[1])if m else 0
   if not 1<=pages<=1000:bad('payload_too_large')
  else:expanded,sheets,cells=office(data,kind)
  pages_total+=pages;sheets_total+=sheets;cells_total+=cells;expanded_total+=expanded
  if pages_total>3000 or sheets_total>200 or cells_total>250000 or expanded_total>250*1048576:bad('payload_too_large')
  note(member={'path':name,'normalized_path':n,'filename':name.split('/')[-1],'kind':kind,'byte_size':len(data),'expanded_bytes':expanded,'pages':pages,'sheets':sheets,'cells':cells,'scanner_revision':revision,'sha256':hashlib.sha256(data).hexdigest(),'data':base64.b64encode(data).decode()});note(path=name,disposition='verified',error_code='')
 if name.lower().endswith('.zip'):
  z,items,_=entries(data,1000,250*1048576)
  for f in items:note(path=name+'/'+f.filename,disposition='listed',error_code='')
  for f in items:
   full=name+'/'+f.filename
   if any(p.lower()in('__macosx','.ds_store')or p.startswith('._')for p in f.filename.split('/')):note(path=full,disposition='ignored',error_code='archive_metadata');continue
   if f.is_dir():note(path=full,disposition='directory',error_code='');continue
   with z.open(f)as s:body=s.read(LEAF+1)
   if len(body)!=f.file_size:bad('archive_integrity_failed')
   leaf(full,body)
 else:leaf(name,data)
 if not members:bad('empty_package')
 note(done=True)
try:run()
except Invalid as e:note(error=str(e))
except Exception:note(error='verification_failed')
