"""Private, single-request CPU inference. No prompts, credentials or text logs."""
import hashlib,json,math,platform
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
from guardrails import InferenceBusy,InferenceGuard
import numpy as np
import onnxruntime as ort
from tokenizers import Tokenizer

MODEL="minilm-l6@233902d25c440f23af6f7d6e94d2946bac0bee0a/onnx-int8-window-v1/"+{"aarch64":"arm64","arm64":"arm64","x86_64":"amd64"}[platform.machine()]
root=Path("/model");manifest=json.loads((root/"manifest.json").read_text())
for name,digest in manifest["files"].items():
 if hashlib.sha256((root/name).read_bytes()).hexdigest()!=digest:raise RuntimeError("model integrity check failed")
options=ort.SessionOptions();options.intra_op_num_threads=1;options.inter_op_num_threads=1
options.execution_mode=ort.ExecutionMode.ORT_SEQUENTIAL
session=ort.InferenceSession(str(root/"model.onnx"),sess_options=options,providers=["CPUExecutionProvider"])
tokenizer=Tokenizer.from_file(str(root/"tokenizer.json"))
query_tokenizer=Tokenizer.from_file(str(root/"tokenizer.json"))
tokenizer.enable_truncation(max_length=512,stride=64,strategy="only_second")
inputs={v.name for v in session.get_inputs()}
guard=InferenceGuard(10)

def score(payload):
 if not isinstance(payload,dict) or set(payload)!={"query","passages"}:raise ValueError()
 query=payload["query"];passages=payload["passages"]
 if not isinstance(query,str) or len(query.encode())>2000 or len(query_tokenizer.encode(query).ids)>128:raise ValueError()
 if not isinstance(passages,list) or not 1<=len(passages)<=30:raise ValueError()
 ids=set();windows=[];owners=[]
 for i,p in enumerate(passages):
  if not isinstance(p,dict) or set(p)!={"id","text"}:raise ValueError()
  if not isinstance(p["id"],str) or not 1<=len(p["id"])<=200 or p["id"] in ids:raise ValueError()
  if not isinstance(p["text"],str) or not 1<=len(p["text"].encode())<=2200:raise ValueError()
  ids.add(p["id"])
  e=tokenizer.encode(query,p["text"])
  parts=[e]+e.overflowing
  if len(parts)>8:raise ValueError() # refuse rather than silently lose source text
  windows.extend(parts);owners.extend([i]*len(parts))
 scores=[-math.inf]*len(passages)
 for start in range(0,len(windows),8):
  batch=windows[start:start+8];length=max(len(e.ids) for e in batch)
  def padded(attr,pad):return np.array([getattr(e,attr)+[pad]*(length-len(e.ids)) for e in batch],dtype=np.int64)
  feed={"input_ids":padded("ids",0),"attention_mask":padded("attention_mask",0),"token_type_ids":padded("type_ids",0)}
  result=session.run(None,{k:v for k,v in feed.items() if k in inputs})[0].reshape(-1)
  for j,v in enumerate(result):
   value=float(v)
   if not math.isfinite(value):raise ValueError()
   owner=owners[start+j];scores[owner]=max(scores[owner],value)
 return {"model":MODEL,"scores":[{"id":p["id"],"score":scores[i]} for i,p in enumerate(passages)]}

class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def setup(self):
  super().setup();self.connection.settimeout(5)
 def reply(self,status,body):
  raw=json.dumps(body,allow_nan=False).encode();self.send_response(status)
  self.send_header("Content-Type","application/json");self.send_header("Content-Length",str(len(raw)));self.send_header("Connection","close");self.end_headers()
  try:self.wfile.write(raw)
  except (BrokenPipeError,ConnectionResetError):pass
 def do_GET(self):
  self.reply(200,{"model":MODEL}) if self.path=="/health" else self.reply(404,{"error":"not_found"})
 def do_POST(self):
  if self.path!="/rerank":self.reply(404,{"error":"not_found"});return
  try:
   length=int(self.headers.get("Content-Length","0"))
   if not 0<length<=131072 or self.headers.get("Transfer-Encoding"):raise ValueError()
   raw=self.rfile.read(length)
   if len(raw)!=length:raise ValueError()
   payload=json.loads(raw)
   self.reply(200,guard.run(lambda:score(payload)))
  except InferenceBusy:self.reply(503,{"error":"local_inference_busy"})
  except (ValueError,TypeError,KeyError,UnicodeError):self.reply(422,{"error":"invalid_or_unbounded_input"})
  except Exception:self.reply(503,{"error":"local_inference_unavailable"})

if __name__=="__main__":ThreadingHTTPServer(("0.0.0.0",8080),Handler).serve_forever()
