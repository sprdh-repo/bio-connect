const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync(require('node:path').join(__dirname,'../internal/app/web/kiosk.js'),'utf8').replace(/void boot\(\);\s*$/,'');
function client(session={}){
  const elements=new Map(),requests=[],timers=new Map();let timerID=0,prints=0;
  const classList={add(){},remove(){},toggle(){},contains(){return false},replace(){}};
  function element(selector){if(!elements.has(selector))elements.set(selector,{value:'',dataset:{},classList,style:{setProperty(){}},listeners:{},addEventListener(n,f){this.listeners[n]=f},focus(){},querySelector:s=>element(s),querySelectorAll:()=>[],replaceChildren(){},append(){},getContext:()=>({drawImage(){}})});return elements.get(selector)}
  const document={querySelector:element,querySelectorAll:()=>[],addEventListener(){},styleSheets:[],documentElement:{},body:element('body')};
  const context=vm.createContext({document,window:{addEventListener(){},print(){prints++}},navigator:{userAgent:''},location:{href:'kiosk'},history:{pushState(){}},localStorage:{getItem:()=>null,setItem(){}},sessionStorage:{getItem:k=>session[k]??null,setItem:(k,v)=>session[k]=v},Intl,Date,Image:function(){return {style:{}}},Badge:{DEFAULT_DPI:203,LABEL_MM:{w:76.2,h:50.8}},crypto:{randomUUID:()=>`request-${++timerID}`},setTimeout:(f,ms)=>{const id=++timerID;timers.set(id,{f,ms});return id},clearTimeout:id=>timers.delete(id),setInterval:()=>++timerID,clearInterval(){},matchMedia:()=>({matches:true}),fetch:(url,options)=>new Promise((resolve,reject)=>requests.push({url,options,resolve,reject}))});
  vm.runInContext(source,context);
  const run=code=>vm.runInContext(code,context);
  const respond=(i,data,status=200)=>requests[i].resolve({ok:status<400,status,json:async()=>data});
  return {context,run,respond,requests,timers,element,prints:()=>prints,session};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const current="state.current={person:{qrId:'qr',name:'Asha'},dayLabel:'Day 1'};state.badge={dataUrl:'png',width:600,height:400};";
test('pending kiosk check-in clears timeout and refuses reset/overlap',async()=>{
  const c=client();c.run(current+"show('welcome',30)");assert.ok(c.timers.size>0);
  const p=c.run('checkIn()');assert.equal(c.timers.size,0);c.run('reset();checkIn()');assert.equal(c.requests.length,1);assert.equal(c.run('state.current.person.qrId'),'qr');
  c.respond(0,{person:{name:'Asha'},dayLabel:'Day 1',repeat:false});await p;assert.equal(c.run('state.screen'),'welcome');assert.equal(c.run('state.busy'),false);
});
test('a kiosk pause suppresses a late check-in success',async()=>{
  const c=client();c.run(current);const p=c.run('checkIn()');c.run("pause('expired')");c.respond(0,{person:{name:'Asha'},dayLabel:'Day 1',repeat:false});await p;assert.equal(c.run('state.screen'),'paused');
});
test('lost print authorization response retries same ID without changing dispatch',async()=>{
  const c=client();c.run(current);const p=c.run("printBadge(false,$('#print-button'))");await tick();
  c.requests[0].reject(new Error('offline'));await p;assert.equal(c.prints(),0);assert.equal(c.element('#problem-retry').hidden,false);
  const q=c.run("printBadge(state.printRecorded,$('#problem-retry'))");await tick();assert.equal(c.requests[0].options.body,c.requests[1].options.body);
  c.respond(1,{checkedIn:true});await q;assert.equal(c.prints(),1);
  const reprint=c.run("printBadge(true,$('#reprint-button'))");await tick();assert.notEqual(JSON.parse(c.requests[2].options.body).requestId,JSON.parse(c.requests[1].options.body).requestId);assert.equal(JSON.parse(c.requests[2].options.body).retry,true);c.respond(2,{checkedIn:true});await reprint;
});
test('lost authorization survives page reload and done scan prepares the same badge',async()=>{
  const c=client();c.run(current);const p=c.run("printBadge(false,$('#print-button'))");await tick();c.requests[0].reject(new Error('offline'));await p;
  const next=client(c.session);next.run("prepareBadge=async()=>{state.badge={dataUrl:'png'}}");const scan=next.run("onCode('qr')");next.respond(0,{status:'done',person:{qrId:'qr',name:'Asha'},dayLabel:'Day 1'});await scan;assert.equal(next.run('state.screen'),'confirm');
  const retry=next.run("printBadge(false,$('#print-button'))");await tick();assert.equal(next.requests[1].options.body,c.requests[0].options.body);next.respond(1,{checkedIn:true});await retry;assert.equal(next.prints(),1);
});
