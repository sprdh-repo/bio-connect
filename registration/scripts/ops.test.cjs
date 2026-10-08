const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync(require('node:path').join(__dirname,'../internal/app/web/ops.js'),'utf8').replace(/bootstrap\(\);\s*$/,'');
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return {promise,resolve,reject}};
function client(){
  const elements=new Map(),requests=[],intervals=[];let prints=0,uuid=0;
  function element(selector){if(!elements.has(selector))elements.set(selector,{value:'',dataset:{},textContent:'',innerHTML:'',hidden:false,disabled:false,listeners:{},classList:{add(){},remove(){},toggle(){}},addEventListener(n,f){this.listeners[n]=f},focus(){document.activeElement=this},blur(){},showModal(){this.open=true},close(){this.open=false},setAttribute(){},append(){},before(){},scrollIntoView(){},remove(){},decode:async()=>{}});return elements.get(selector)}
  const document={querySelector:element,querySelectorAll:s=>s==='#scan-form input, #scan-form button'?[element('#scan-code'),element('#camera-open'),element('submit')]:[],createElement:()=>element('badge'),body:element('body'),activeElement:null};
  const context=vm.createContext({document,window:{addEventListener(){},print(){prints++}},navigator:{},location:{hash:''},history:{replaceState(){}},sessionStorage:{},innerWidth:1000,Intl,Date,Image:function(){return element('image')},Badge:{png:async()=> 'png'},crypto:{randomUUID:()=>`id-${++uuid}`},setTimeout:()=>0,clearTimeout(){},setInterval:f=>intervals.push(f),confirm:()=>true,prompt:()=> 'reason',fetch:(url,options)=>{const d=deferred();requests.push({url,options,...d});return d.promise}});
  vm.runInContext(source,context);
  const run=code=>vm.runInContext(code,context);
  function respond(i,data,status=200){requests[i].resolve({ok:status<400,status,headers:{get:()=> 'application/json'},json:async()=>data})}
  return {run,element,requests,respond,intervals,prints:()=>prints,context};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
test('gate creation displays all registration types and submits the selected filter',async()=>{
  const c=client();c.context.FormData=class{get(name){return {name:'Restricted gate',mode:'enforce',direction:'entry',capacity:''}[name]}getAll(name){return name==='registrationType'?['complimentary','free_link']:['student']}has(){return true}};
  c.run("state.view='gates';showGateForm()");
  const html=c.element('#gate-create').innerHTML;
  assert.match(html,/Allowed registration types/);
  for(const type of ['paid','complimentary','free_link'])assert.ok(html.includes(`name="registrationType" value="${type}" checked`));
  const saving=c.element('#gate-form').listeners.submit({preventDefault(){},target:{},submitter:c.element('create')});
  const body=JSON.parse(c.requests[0].options.body);
  assert.deepEqual(body.allowedRegistrationTypes,['complimentary','free_link']);
  assert.deepEqual(body.allowedCategories,['student']);
  c.respond(0,{point:{}});await tick();c.respond(1,{points:[]});await saving;
});
test('print preparation cannot overlap and audit keeps its captured day',async()=>{
  const c=client(),png=deferred();c.context.Badge.png=()=>png.promise;
  const first=c.run("printBadge({qrId:'a',name:'A'},'qr')");
  await c.run("printBadge({qrId:'b',name:'B'},'qr')");
  c.run("state.day='2026-10-09'");png.resolve('png');await tick();
  assert.equal(c.prints(),0,'obsolete preparation must not dispatch');await first;
  const next=c.run("printBadge({qrId:'a',name:'A'},'qr')");await tick();
  assert.equal(c.prints(),1);c.run("state.day='2026-10-08'");
  assert.equal(JSON.parse(c.requests[0].options.body).day,'2026-10-09');c.respond(0,{});await next;
});
test('desk scans serialize and stale responses do not print or change results',async()=>{
  const c=client();c.run("bindScanner('check-in')");const form=c.element('#scan-form'),input=c.element('#scan-code');input.value='a';
  const submit=()=>form.listeners.submit({preventDefault(){},submitter:c.element('submit')});submit();submit();
  assert.equal(c.requests.length,1);assert.equal(input.disabled,true);
  c.run("state.day='2026-10-09';state.epoch++");c.respond(0,{person:{qrId:'a',name:'A'},qrUrl:'qr'});await tick();
  assert.equal(c.prints(),0);assert.equal(c.element('#scan-result').innerHTML,'');assert.equal(JSON.parse(c.requests[0].options.body).day,'2026-10-08');
});
async function gateClient(){const c=client();c.run("state.view='gates'");const opening=c.run("operateGate({id:'g',name:'Gate',mode:'enforce',active:true,capacity:10})");c.respond(0,{insideCount:0,inside:[]});await opening;return c}
function submit(c,code='a'){c.element('#scan-code').value=code;c.element('#scan-form').listeners.submit({preventDefault(){},submitter:c.element('submit')})}
test('gate scan labels distinguish paid, complimentary and free link on grants and denials',async()=>{
  for(const [type,label] of [['paid','Paid'],['complimentary','Complimentary'],['free_link','Free link']]){
    for(const allowed of [true,false]){
      const c=await gateClient();submit(c);c.respond(1,{allowed,insideCount:1,person:{name:'Attendee',registrationType:type}});await tick();
      assert.ok(c.element('#scan-result').innerHTML.includes('Registration: '+label));
      assert.ok(c.element('#scan-result').innerHTML.includes(allowed?'ACCESS GRANTED':'ACCESS DENIED'));
    }
  }
});
test('registration filter saves only selected types without toggling the gate',async()=>{
  const c=await gateClient();c.context.FormData=class{getAll(name){assert.equal(name,'registrationType');return ['complimentary','free_link']}};
  const saving=c.element('#gate-registration-form').listeners.submit({preventDefault(){},target:{},submitter:c.element('save')});
  assert.deepEqual(JSON.parse(c.requests[1].options.body),{allowedRegistrationTypes:['complimentary','free_link']});
  c.respond(1,{point:{allowedRegistrationTypes:['complimentary','free_link'],active:true}});await saving;
  assert.equal(c.element('#gate-registration-summary').textContent,'Current: Complimentary · Free link');
});
test('gate transport retry uses one logical request ID and captured day',async()=>{
  const c=await gateClient();submit(c);submit(c,'b');assert.equal(c.requests.length,2);
  c.requests[1].reject(new TypeError('network'));await tick();assert.equal(c.requests.length,3);
  assert.equal(c.requests[1].options.body,c.requests[2].options.body);
  const body=JSON.parse(c.requests[2].options.body);assert.equal(body.requestId,'id-1');assert.equal(body.day,'2026-10-08');
  c.run("state.day='2026-10-09';state.epoch++");c.respond(2,{allowed:true,insideCount:1});await tick();
  assert.equal(c.element('#scan-result').innerHTML,'');assert.equal(c.requests.length,3);assert.equal(c.element('#scan-code').disabled,false);
});
test('gate HTTP errors are not retried, and later scans have fresh IDs',async()=>{
  for(const status of [400,403,409,500,503]){const c=await gateClient();submit(c);c.respond(1,{error:'denied'},status);await tick();assert.equal(c.requests.length,2);assert.match(c.element('#scan-result').innerHTML,/denied/);submit(c,'b');assert.equal(JSON.parse(c.requests[2].options.body).requestId,'id-2');c.respond(2,{error:'denied'},status);await tick()}
});
test('gate retries only once on transport failure and unlocks controls',async()=>{
  const c=await gateClient();submit(c);c.requests[1].reject(new Error('offline'));await tick();c.requests[2].reject(new Error('offline'));await tick();assert.equal(c.requests.length,3);assert.equal(c.element('#camera-open').disabled,false);
});
test('roster polling coalesces loads and keeps current search and focus',async()=>{
  const c=client();c.run("state.view='roster';renderRoster()");const search=c.element('#roster-search');search.value='alice';search.focus();
  const first=c.intervals[0]();await c.intervals[0]();const manual=c.run('refreshRoster()');assert.equal(c.requests.length,1);
  c.respond(0,{people:[{name:'Alice',qrId:'a'},{name:'Bob',qrId:'b'}]});await Promise.all([first,manual]);
  assert.equal(search.value,'alice');assert.equal(c.context.document.activeElement,search);assert.match(c.element('#roster-body').innerHTML,/Alice/);assert.doesNotMatch(c.element('#roster-body').innerHTML,/Bob/);
  search.value='bob';search.listeners.input();assert.match(c.element('#roster-body').innerHTML,/Bob/);
});
test('late roster and render responses cannot replace a newer day or view',async()=>{
  const c=client();c.run("state.view='roster'");const first=c.run('render()');c.run("state.day='2026-10-09'");const next=c.run('render()');
  c.respond(1,{people:[{name:'Today'}]});await next;const html=c.element('#workspace').innerHTML;
  c.respond(0,{people:[{name:'Yesterday'}]});await first;assert.equal(c.element('#workspace').innerHTML,html);assert.equal(c.run('state.roster[0].name'),'Today');
  const poll=c.intervals[0]();c.run("navigate('checkout')");const checkout=c.element('#workspace').innerHTML;c.respond(2,{people:[{name:'Obsolete'}]});await poll;assert.equal(c.element('#workspace').innerHTML,checkout);
});
test('live occupancy updates list and count but ignores stale gate and scan snapshots',async()=>{
  const c=await gateClient(),poll=c.intervals[0]();submit(c);c.respond(1,{insideCount:9,inside:[{name:'Old'}]});await poll;assert.equal(c.element('#gate-count').textContent,'');
  c.respond(2,{allowed:true,direction:'entry',insideCount:1,person:{name:'New'}});await tick();assert.equal(c.requests.length,4);
  c.respond(3,{insideCount:1,inside:[{name:'New'}]});await tick();assert.match(c.element('#gate-count').textContent,/1 \/ 10/);assert.match(c.element('#gate-inside').innerHTML,/New/);
  const late=c.intervals[0]();c.run("navigate('checkout')");c.element('#gate-count').textContent='sentinel';c.respond(4,{insideCount:99,inside:[]});await late;assert.equal(c.element('#gate-count').textContent,'sentinel');
});
test('late gate toggle cannot modify another screen',async()=>{
  const c=await gateClient(),toggle=c.element('#gate-toggle').onclick();c.run("navigate('checkout')");const html=c.element('#workspace').innerHTML;c.respond(1,{});await toggle;assert.equal(c.element('#workspace').innerHTML,html);assert.equal(c.requests.length,2);
});
test('undo carries scanned day and ignores a late result after navigation',async()=>{
  const c=client();c.run("showPerson({person:{qrId:'a',name:'A'}},'check-in')");const undo=c.element('#undo').listeners.click();assert.equal(JSON.parse(c.requests[0].options.body).day,'2026-10-08');
  c.run("state.day='2026-10-09';navigate('checkout')");c.element('#scan-result').innerHTML='new screen';c.respond(0,{});await undo;assert.equal(c.element('#scan-result').innerHTML,'new screen');assert.equal(c.requests.length,1);
});
test('camera callbacks and keyboard share the same pending scan guard',async()=>{
  const c=client();let decoded;
  c.context.navigator.mediaDevices={getUserMedia(){}};
  c.context.Html5Qrcode=class {async start(a,b,callback){decoded=callback}async stop(){}async clear(){}};
  c.run("bindScanner('check-out')");await c.run("openCamera(code=>post('check-out',{code,day:state.day}))");submit(c);await decoded('camera');assert.equal(c.requests.length,1);
  c.run("navigate('checkout')");await decoded('stale');assert.equal(c.requests.length,1);c.respond(0,{person:{name:'A'}});await tick();
});
test('print lock lasts through decode, dispatch and audit, then releases on failure',async()=>{
  const c=client(),decode=deferred();c.element('image').decode=()=>decode.promise;
  const print=c.run("printBadge({qrId:'a',name:'A'},'qr')");await tick();await c.run("printBadge({qrId:'b'},'qr')");assert.equal(c.prints(),0);
  decode.resolve();await tick();await c.run("printBadge({qrId:'b'},'qr')");assert.equal(c.prints(),1);assert.equal(c.requests.length,1);
  c.respond(0,{error:'audit failed'},500);await print;assert.equal(c.run('state.printPending'),false);
});
test('failed new-day roster load cannot show previous-day statuses',async()=>{
  const c=client();c.run("state.view='roster';state.rosterDay='2026-10-08';state.roster=[{name:'Yesterday'}];state.day='2026-10-09'");
  const p=c.run('render()');c.respond(0,{error:'offline'},503);await p;assert.equal(c.run('state.roster.length'),0);assert.doesNotMatch(c.element('#roster-body').innerHTML,/Yesterday/);
});
test('occupancy polling reflects policy changes from another gate station',async()=>{
  const c=await gateClient(),p=c.intervals[0]();c.respond(1,{insideCount:1,inside:[],point:{active:false,mode:'enforce',capacity:5}});await p;
  assert.equal(c.element('#gate-mode').textContent,'enforce mode · Closed');assert.equal(c.element('#gate-toggle').textContent,'Open gate');assert.match(c.element('#gate-count').textContent,/1 \/ 5/);
});
test('roster filters by category and matches category in text search',()=>{
  const c=client();
  c.run(`state.categories=[{id:'student',label:'Student'},{id:'faculty',label:'Faculty'}];state.roster=[{name:'Asha',category:'Student',reference:'S1',qrId:'s1'},{name:'Binu',category:'Faculty',reference:'F1',qrId:'f1'}];renderRoster()`);
  assert.match(c.element('#workspace').innerHTML,/<option value="">All categories<\/option><option>Student<\/option><option>Faculty<\/option>/);
  c.element('#roster-category').value='Faculty';c.element('#roster-category').listeners.change();
  assert.match(c.element('#roster-body').innerHTML,/Binu/);assert.doesNotMatch(c.element('#roster-body').innerHTML,/Asha/);
  c.element('#roster-category').value='';c.element('#roster-search').value='stud';c.element('#roster-search').listeners.input();
  assert.match(c.element('#roster-body').innerHTML,/Asha/);assert.doesNotMatch(c.element('#roster-body').innerHTML,/Binu/);
});
