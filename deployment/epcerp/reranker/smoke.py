"""Synthetic real-model smoke test; never claims real tender accuracy."""
import json,urllib.request
payload={"query":"What is the earnest money deposit amount?","passages":[{"id":"fee","text":"The tender document fee is INR 2000, payable online."},{"id":"emd","text":"Earnest money deposit of INR 500000 is required with the bid."},{"id":"time","text":"The installation must be completed within 90 days."}]}
request=urllib.request.Request("http://127.0.0.1:8080/rerank",data=json.dumps(payload).encode(),headers={"Content-Type":"application/json"})
with urllib.request.urlopen(request,timeout=20) as r:result=json.load(r)
scores={v["id"]:v["score"] for v in result["scores"]}
assert scores["emd"]>scores["fee"] and scores["emd"]>scores["time"],"Synthetic EMD relevance check failed"
print("Pinned local model: synthetic EMD-vs-fee ranking passed; no cloud call.")
