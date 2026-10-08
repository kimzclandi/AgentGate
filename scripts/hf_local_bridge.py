"""Optional loopback Ollama-wire adapter using a real local Transformers model.
No fabricated tool calls: only parses model-produced <tool_call> JSON. Evaluation
fixture, single-process CPU. This does not claim to run Ollama or Qwen3.
"""
import argparse,json,re,threading
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import torch
from transformers import AutoTokenizer,AutoModelForCausalLM
p=argparse.ArgumentParser();p.add_argument("--snapshot",required=True);a=p.parse_args()
torch.set_num_threads(4)
tokenizer=AutoTokenizer.from_pretrained(a.snapshot,local_files_only=True)
model=AutoModelForCausalLM.from_pretrained(a.snapshot,local_files_only=True,torch_dtype=torch.float32,attn_implementation="eager").eval()
lock=threading.Lock()
class Handler(BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_POST(self):
        if self.path!="/api/chat":self.send_error(404);return
        size=int(self.headers.get("Content-Length",0))
        if not 0<size<=65536:self.send_error(413);return
        body=json.loads(self.rfile.read(size));messages=body["messages"]
        # Qwen's template expects a dictionary for function arguments.
        for m in messages:
            if "tool_name" in m:m["name"]=m["tool_name"]
        text=tokenizer.apply_chat_template(messages,tools=body["tools"],tokenize=False,add_generation_prompt=True)
        with lock,torch.inference_mode():
            x=tokenizer(text,return_tensors="pt");y=model.generate(**x,do_sample=False,max_new_tokens=256)
            raw=tokenizer.decode(y[0,x["input_ids"].shape[1]:],skip_special_tokens=True)
        calls=[]
        try:
            for value in re.findall(r"<tool_call>\s*(.*?)\s*</tool_call>",raw,re.S):
                v=json.loads(value);calls.append({"function":{"name":v["name"],"arguments":v["arguments"]}})
        except (ValueError,KeyError):calls=[]
        result={"done":True,"message":{"role":"assistant","content":re.sub(r"<tool_call>.*?</tool_call>","",raw,flags=re.S).strip(),"tool_calls":calls}}
        data=json.dumps(result).encode();self.send_response(200);self.send_header("Content-Type","application/json");self.send_header("Content-Length",str(len(data)));self.end_headers()
        try:self.wfile.write(data)
        except BrokenPipeError:pass
        print(json.dumps({"raw":raw,"calls":calls}),flush=True)
print("READY real Transformers Qwen CPU adapter on 127.0.0.1:11434",flush=True)
ThreadingHTTPServer(("127.0.0.1",11434),Handler).serve_forever()
