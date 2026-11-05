"""Offline table geometry only. No network, credentials, document instructions or logs."""
import argparse,json,sys
import pdfplumber
from pdfplumber.utils import extract_text

MAX_PAGES=100
MAX_CELLS=12000
HEADER_WORDS={'description','quantity','qty','unit','rate','amount','remarks','confirmation','documents','uploading','particulars','s.no.','s.no','item'}
def extract(path):
 out={'version':'pdf-tables-v1','pages':[]};count=0
 with pdfplumber.open(path) as pdf:
  if len(pdf.pages)>MAX_PAGES:raise ValueError('page_limit')
  for number,page in enumerate(pdf.pages,1):
   # Some generators draw continuing cells past the visible media box. Preserve
   # only this page's visible geometry; never invent the next page's contents.
   page=page.crop(page.bbox)
   tables=[]
   for ti,table in enumerate(page.find_tables()):
    if len(table.cells)<2:continue
    xs=sorted(set(x for b in table.cells for x in (b[0],b[2])))
    ys=sorted(set(y for b in table.cells for y in (b[1],b[3])))
    cells=[]
    for box in table.cells:
     x0,y0,x1,y1=box
     chars=[c for c in page.chars if x0<=(c['x0']+c['x1'])/2<x1 and y0<=(c['top']+c['bottom'])/2<y1]
     text=extract_text(chars,x_tolerance=2,y_tolerance=3) if chars else ''
     if len(text.encode())>12000:raise ValueError('cell_text_limit')
     cells.append({'row':ys.index(y0),'column':xs.index(x0),'row_span':ys.index(y1)-ys.index(y0),'column_span':xs.index(x1)-xs.index(x0),'text':text,'x':x0-page.bbox[0],'y':y0-page.bbox[1],'width':x1-x0,'height':y1-y0})
    first=[c for c in cells if c['row']==0]
    header= bool(first) and all(len(c['text'])<100 for c in first) and sum(any(w in c['text'].lower().split() for w in HEADER_WORDS) for c in first)>=2
    for c in cells:
     c['header']=bool(header and c['row']==0)
     c['column_header']=' | '.join(h['text'].replace('\n',' ') for h in first if header and h['column']<=c['column']<h['column']+h['column_span']) if c['row']>0 else ''
    count+=len(cells)
    if count>MAX_CELLS:raise ValueError('cell_count_limit')
    tables.append({'id':f'p{number}-t{ti+1}','cells':cells})
   out['pages'].append({'page':number,'width':page.width,'height':page.height,'tables':tables})
 return out
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--input',required=True);args=parser.parse_args()
 try:
  raw=json.dumps(extract(args.input),ensure_ascii=False,separators=(',',':')).encode()
  if len(raw)>8*1024*1024:raise ValueError('output_limit')
  sys.stdout.buffer.write(raw)
 except Exception:
  # The caller emits a fixed coverage warning. Never expose PDF contents/errors.
  sys.exit(1)
