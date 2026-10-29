"""Build-time only: obtain pinned public weights; never receive tender data."""
import hashlib,json,os,subprocess
from pathlib import Path
revision="233902d25c440f23af6f7d6e94d2946bac0bee0a"
arch=os.environ["TARGETARCH"]
name={"arm64":"model_qint8_arm64.onnx","amd64":"model_quint8_avx2.onnx"}[arch]
root=Path("/model");root.mkdir(exist_ok=True)
files={"tokenizer.json":"tokenizer.json","model.onnx":"onnx/"+name}
manifest={"revision":revision,"arch":arch,"files":{}}
for target,source in files.items():
 path=root/target
 subprocess.run(["curl","--fail","--silent","--show-error","--location","--retry","2","--output",str(path),"https://huggingface.co/cross-encoder/ms-marco-MiniLM-L6-v2/resolve/"+revision+"/"+source],check=True)
 manifest["files"][target]=hashlib.sha256(path.read_bytes()).hexdigest()
(root/"manifest.json").write_text(json.dumps(manifest,sort_keys=True))
