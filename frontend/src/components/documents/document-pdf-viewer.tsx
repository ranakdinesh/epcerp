"use client";
import { useEffect, useRef, useState, useId } from "react";
import type { PDFDocumentProxy, RenderTask } from "pdfjs-dist";

export function DocumentPDFViewer({url,filename}:{url:string;filename:string}) {
 const textID=useId();
 const [pdf,setPdf]=useState<PDFDocumentProxy|null>(null),[page,setPage]=useState(1),[zoom,setZoom]=useState(1),[width,setWidth]=useState(800),[error,setError]=useState(""),[rendering,setRendering]=useState(true),[pageText,setPageText]=useState("");
 const container=useRef<HTMLDivElement>(null),canvas=useRef<HTMLCanvasElement>(null);
 useEffect(()=>{const node=container.current;if(!node)return;const observer=new ResizeObserver(entries=>setWidth(Math.max(240,Math.min(1400,entries[0].contentRect.width-24))));observer.observe(node);return()=>observer.disconnect();},[]);
 useEffect(()=>{let cancelled=false;let loading:ReturnType<typeof import("pdfjs-dist").getDocument>|undefined;
  import("pdfjs-dist").then(library=>{if(cancelled)return;const assets=`/pdfjs-${library.version}/`;library.GlobalWorkerOptions.workerSrc=`${assets}pdf.worker.min.mjs`;
   // Render document content only. No document JavaScript, XFA actions or external links.
   loading=library.getDocument({url,verbosity:0,maxImageSize:40000000,enableXfa:false,standardFontDataUrl:`${assets}standard_fonts/`,cMapUrl:`${assets}cmaps/`,cMapPacked:true,wasmUrl:`${assets}wasm/`});
   return loading.promise.then(document=>{if(!cancelled)setPdf(document);});
  }).catch(()=>{if(!cancelled)setError("This PDF cannot be rendered. Download it for inspection or upload a valid replacement.");});
  return()=>{cancelled=true;void loading?.destroy().catch(()=>{});};
 },[url]);
 useEffect(()=>{if(!pdf)return;let cancelled=false;let task:RenderTask|undefined;
  pdf.getPage(page).then(async document=>{if(cancelled||!canvas.current)return;setRendering(true);setError("");const base=document.getViewport({scale:1});const ratio=Math.min(window.devicePixelRatio||1,2);const scale=Math.min(width/base.width*zoom,Math.sqrt(8000000/(base.width*base.height))/ratio,16000/(base.height*ratio));const viewport=document.getViewport({scale});const node=canvas.current;const context=node.getContext("2d");if(!context)throw new Error("Canvas unavailable");node.width=Math.ceil(viewport.width*ratio);node.height=Math.ceil(viewport.height*ratio);node.style.width=`${viewport.width}px`;node.style.height=`${viewport.height}px`;task=document.render({canvas:node,canvasContext:context,viewport,transform:ratio===1?undefined:[ratio,0,0,ratio,0,0]});await task.promise;if(cancelled)return;const text=await document.getTextContent();if(!cancelled){setPageText(text.items.map(item=>"str" in item?item.str:"").join(" "));setRendering(false);}
  }).catch(()=>{if(!cancelled){setError("Unable to render this page. Try downloading the PDF.");setRendering(false);}});
  return()=>{cancelled=true;task?.cancel();};
 },[pdf,page,zoom,width]);
 return <section className="space-y-3" aria-label={`PDF viewer: ${filename}`}><div className="flex flex-wrap items-center justify-between gap-2"><div className="flex items-center gap-2"><button type="button" className="min-h-10 rounded border px-3" disabled={!pdf||page===1} onClick={()=>setPage(v=>v-1)}>Previous page</button><span className="text-sm">Page {page} of {pdf?.numPages??"…"}</span><button type="button" className="min-h-10 rounded border px-3" disabled={!pdf||page>=pdf.numPages} onClick={()=>setPage(v=>v+1)}>Next page</button></div><div className="flex items-center gap-2"><button type="button" aria-label="Zoom out" className="min-h-10 min-w-10 rounded border" disabled={zoom<=0.5} onClick={()=>setZoom(v=>Math.max(0.5,v-0.25))}>−</button><span className="text-sm">{Math.round(zoom*100)}%</span><button type="button" aria-label="Zoom in" className="min-h-10 min-w-10 rounded border" disabled={zoom>=2} onClick={()=>setZoom(v=>Math.min(2,v+0.25))}>+</button><a className="min-h-10 px-2 py-2 text-blue-700" href={url} download={filename}>Download PDF</a></div></div>{error&&<p role="alert" className="text-red-700">{error}</p>}{!error&&rendering&&<p role="status">Rendering PDF page…</p>}<div ref={container} className="max-h-[65dvh] overflow-auto rounded-lg border bg-slate-200 p-3"><canvas ref={canvas} role="img" aria-label={`PDF page ${page} of ${pdf?.numPages??"unknown"}`} aria-describedby={textID} className="mx-auto bg-white shadow"/><p id={textID} className="sr-only">{pageText}</p></div></section>;
}
