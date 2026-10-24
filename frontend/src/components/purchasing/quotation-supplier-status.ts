export function quotationSupplierSummary(status?:{state:string;message:string}):{label:string;detail:string;approved:boolean}{
 if(status?.state==="verified")return {label:"Approved",detail:"",approved:true};
 const missing=status?.message?.match(/^Required evidence missing:\s*(.+)$/i);
 if(missing){const names:Record<string,string>={pan:"PAN",cheque:"cancelled cheque",gst:"GST",aadhar:"Aadhaar"};return {label:"Missing documents",detail:missing[1].split(",").map(v=>names[v.trim()]??v.trim()).join(", "),approved:false};}
 if(status?.state==="rejected")return {label:"Rejected",detail:"Replace or review rejected files",approved:false};
 if(status?.state==="blocked")return {label:"Needs attention",detail:status.message,approved:false};
 if(status?.state==="unverified")return {label:"Review pending",detail:"Document approval is pending",approved:false};
 return {label:"Status unavailable",detail:"Refresh supplier status",approved:false};
}
