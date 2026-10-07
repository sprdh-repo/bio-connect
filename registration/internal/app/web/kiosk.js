'use strict';
// Self-service kiosk: scan a pass QR, confirm, print the badge (which checks the
// attendee in), then point them to the lanyard. The server decides what each
// pass may do; this page only renders the badge and drives the screens.
const $=(s,p=document)=>p.querySelector(s), $$=(s,p=document)=>[...p.querySelectorAll(s)];

const isAndroid=/Android/i.test(navigator.userAgent);
const DEFAULTS={printer:isAndroid?'rawbt':'browser',dpi:Badge.DEFAULT_DPI,rotate:0,camera:'user'};
const RAWBT_PACKAGE='ru.a402d.rawbtprinter';

// The Bio Connect Kiosk Android app injects BioConnectKiosk (an origin-locked
// web message channel) and prints straight to the Zebra. When it is present it
// replaces RawBT and the browser dialog, and its printer's dpi wins.
const appBridge=(()=>{
  const port=window.BioConnectKiosk;
  if(!port||typeof port.postMessage!=='function')return null;
  const jobs=new Map(),waiting=[];let info=null;
  port.onmessage=e=>{
    let m;try{m=JSON.parse(e.data)}catch{return}
    if(m.type==='info'){info=m;waiting.splice(0).forEach(done=>done(m));showPrinterStatus()}
    else if(m.type==='printed'){const job=jobs.get(m.jobId);if(job){jobs.delete(m.jobId);clearTimeout(job.timer);job.done(m)}}
  };
  const send=m=>port.postMessage(JSON.stringify(m));
  return {
    get printer(){return info?.printer||null},
    // Resolves with the freshest printer info, or the last known after a timeout.
    status(timeout=4000){return new Promise(done=>{const t=setTimeout(()=>done(info),timeout);waiting.push(m=>{clearTimeout(t);done(m)});send({type:info?'status':'hello'})})},
    print(badge,timeout=20000){
      const jobId=crypto.randomUUID();
      return new Promise(done=>{
        const timer=setTimeout(()=>{jobs.delete(jobId);done({ok:false,message:'The printer did not respond'})},timeout);
        jobs.set(jobId,{done,timer});
        send({type:'print',jobId,png:badge.dataUrl.slice(badge.dataUrl.indexOf(',')+1),widthDots:badge.width,heightDots:badge.height});
      });
    }
  };
})();
function printerMode(){return appBridge?'app':settings.printer}
function badgeDpi(){const d=appBridge?.printer?.dpi;return d===203||d===300?d:Number(settings.dpi)||Badge.DEFAULT_DPI}
function badgeRotated(){return !appBridge&&Number(settings.rotate)===90}
function printerError(message,recorded){return Object.assign(new Error(String(message||'The badge did not print').replace(/\.$/,'')),{code:'printer',recorded})}
// Staff need to see a printer problem before an attendee finds it.
function showPrinterStatus(){
  const p=appBridge?.printer,bad=!!p&&(!p.connected||!p.ready);
  $('#printer-status').hidden=!bad;
  if(bad){const why=p.problem||'not connected';$('#printer-status-text').textContent=/^printer\b/i.test(why)?why:'Printer: '+why}
  const box=$('#app-printer');
  if(box&&appBridge){box.hidden=false;box.classList.toggle('problem',!p||bad);box.textContent=!p?'Bio Connect app: checking the printer…':`${p.name||'Badge printer'} · ${p.connection||'connected'} · ${p.dpi||'?'} dpi · ${bad?(p.problem||'not connected'):'Ready'}`}
}
const LABEL_MM=Badge.LABEL_MM;
// Seconds before an unattended screen returns to the scanner.
const IDLE={confirm:45,welcome:30,collect:20,done:12,problem:12,printer:40,staff:90};
const HONORIFICS=/^(dr|prof|mr|mrs|ms|miss|shri|smt|sri|er|adv)\.?$/i;

const state={kiosk:false,csrf:'',station:'',today:'',days:[],screen:'scan',busy:false,paused:false,current:null,badge:null,scanner:null,cameraRetry:null,lastCode:'',holdUntil:0,timer:null,ticker:null,deadline:0,staffPass:'',staffTimer:null,audio:null,wakeLock:null,pageRule:-1};

const store={
  get(key,fallback){try{const v=localStorage.getItem(key);return v===null?fallback:JSON.parse(v)}catch{return fallback}},
  set(key,value){try{localStorage.setItem(key,JSON.stringify(value))}catch{}}
};
let settings={...DEFAULTS,...store.get('bcKioskSettings',{})};

async function api(path,body){
  const headers={'Content-Type':'application/json'};
  if(state.csrf)headers['X-CSRF-Token']=state.csrf;
  let r;
  try{r=await fetch('/api/v1/ops/'+path,{method:body===undefined?'GET':'POST',headers,body:body===undefined?undefined:JSON.stringify(body),cache:'no-store'})}
  catch{const e=new Error('offline');e.status=0;e.code='offline';throw e}
  let data={};try{data=await r.json()}catch{}
  if(!r.ok){const e=new Error(data.error||'Request failed');e.status=r.status;e.code=data.code||'';if(r.status===401&&path!=='kiosk/unlock'&&path!=='kiosk/exit')pause('expired');throw e}
  return data;
}

/* ---------- Screens ---------- */

function firstName(name){const parts=String(name||'').trim().split(/\s+/);const first=parts.find(p=>!HONORIFICS.test(p))||parts[0]||'';return first.charAt(0).toUpperCase()+first.slice(1)}
function setField(name,value){$$(`[data-field="${name}"]`).forEach(el=>el.textContent=value??'')}
function fillPerson(p,dayLabel){
  setField('first',firstName(p.name));setField('name',p.name);setField('designation',p.designation);setField('institution',p.institution);
  setField('category',p.category);setField('reference',p.reference);setField('day',dayShort(dayLabel));
  $$('[data-optional]').forEach(el=>el.hidden=!p[el.dataset.optional]);
}
function dayShort(label){return String(label||'').split('·')[0].trim()||'today'}

function show(name,idle=0){
  clearIdle();
  state.screen=name;
  for(const s of $$('.screen')){
    const on=s.dataset.screen===name;
    if(on&&s.classList.contains('active')){s.classList.remove('active');void s.offsetWidth}
    s.classList.toggle('active',on);s.inert=!on;
  }
  if(idle)armIdle(idle);
}
function clearIdle(){clearTimeout(state.timer);clearInterval(state.ticker);state.timer=state.ticker=null;$$('[data-countdown]').forEach(el=>el.textContent='')}
// Arms (or re-arms after a touch) the countdown back to the scanner.
function armIdle(seconds){
  clearIdle();
  const screen=$(`.screen[data-screen="${state.screen}"]`),bar=$('.timeout span',screen);
  screen.style.setProperty('--timeout',seconds+'s');
  if(bar){bar.style.animation='none';void bar.offsetWidth;bar.style.animation=''}
  state.deadline=Date.now()+seconds*1000;
  const tick=()=>{const left=Math.max(0,Math.ceil((state.deadline-Date.now())/1000));$$('[data-countdown]',screen).forEach(el=>el.textContent=left+'s')};
  tick();state.ticker=setInterval(tick,1000);
  state.timer=setTimeout(reset,seconds*1000);
}
// holdMs: how long the code just handled is ignored once the scanner is back.
// A finished flow holds it briefly so a phone still raised does not restart
// it; "This isn't me" and "Try again" hold it for less, or not at all.
const HOLD_MS=3000;
function reset(holdMs=HOLD_MS){
  clearIdle();
  state.current=null;state.badge=null;state.busy=false;state.printRecorded=false;
  state.holdUntil=Date.now()+(Number.isFinite(holdMs)?holdMs:HOLD_MS);
  $('#reprint-button').hidden=true;
  if(state.paused)return show('paused');
  show('scan');
}

function problem(err){
  const code=err?.code||'',art=$('#problem-art');
  const copy={
    not_found:['Pass not found',"We couldn't find that pass.",'Show the QR code from your Bio Connect 4.0 pass email, WhatsApp message or the app. Still stuck? The registration desk will sort it out.','notfound'],
    qr_required:['Not a pass QR',"That code isn't a Bio Connect pass.",'Use the QR code on your Bio Connect 4.0 pass, from your email, WhatsApp or the app.','notfound'],
    already_printed:['Badge already printed','Your badge was printed earlier.','For a replacement badge, please visit the registration desk. They will print one for you.','helpdesk'],
    offline:['Connection problem',"We couldn't reach the check-in server.",'Please try again in a moment. If it keeps happening, the registration desk will help.','helpdesk']
  }[code]||['Something went wrong','We could not finish that.','Please try again. If it keeps happening, the registration desk will help.','helpdesk'];
  setField('problem-eyebrow',copy[0]);setField('problem-title',copy[1]);setField('problem-copy',copy[2]);setField('problem-reset','Try again');
  $('#problem-retry').hidden=true;$('#problem-reset').classList.replace('btn-ghost','btn-primary');
  art.src=`/static/kiosk/${copy[3]}.webp`;
  // A connection problem is worth retrying with the same code straight away.
  if(code==='offline'||!code)state.lastCode='';
  feedback('reject');
  show('problem',IDLE.problem);
}

function pause(reason){
  state.paused=true;
  const staff=reason==='staff';
  const copy={
    expired:['Kiosk paused','This kiosk is taking a break.','Please visit the registration desk. A staff member will be with you shortly.'],
    setup:['Kiosk not started','Set up this kiosk.','Sign this tablet in as a station in the operations portal, then start the self-service kiosk from there.'],
    staff:['Kiosk not started','Start the self-service kiosk.',`This tablet is signed in as ${state.station}. Starting the kiosk locks it to self check-in until staff unlock it with the passcode.`]
  }[reason]||['Kiosk paused','This kiosk is taking a break.','Please visit the registration desk.'];
  setField('paused-eyebrow',copy[0]);setField('paused-title',copy[1]);setField('paused-copy',copy[2]);
  $('#setup-actions').hidden=false;
  $('#start-button').hidden=!staff;
  if(state.screen!=='paused')show('paused');
}
function resume(){if(!state.paused)return;state.paused=false;reset()}

/* ---------- Scanning ---------- */

async function onCode(raw){
  const code=String(raw||'').trim(),now=Date.now();
  if(!code||state.screen!=='scan'||state.busy||state.paused||$('#staff-dialog').open)return;
  if(code===state.lastCode&&now<state.holdUntil)return;
  state.lastCode=code;state.busy=true;
  $('.scan-panel').classList.add('checking');
  feedback('tick');
  try{
    const x=await api('kiosk/scan',{code});
    state.current=x;state.printRecorded=false;
    fillPerson(x.person,x.dayLabel);
    if(x.status==='print'){
      await prepareBadge(x.person,x.qrUrl);
      feedback('success');
      show('confirm',IDLE.confirm);
    }else showWelcome(x.status,x);
  }catch(err){if(err.status!==401)problem(err)}
  finally{state.busy=false;$('.scan-panel').classList.remove('checking')}
}

function showWelcome(kind,x){
  const day=dayShort(x.dayLabel),first=firstName(x.person.name);
  const lost='Lost your badge? The registration desk can print a replacement.';
  const copy={
    'check-in':['Welcome back',`Good to see you again, ${first}.`,`You already have your badge. Check in for ${day} and head straight in.`,lost],
    done:['Already checked in',`You're all set, ${first}.`,`You're already checked in for ${day}. Wear your badge and head in.`,lost],
    checked:[`Checked in · ${day}`,`You're all set, ${first}.`,'Enjoy the summit. Please keep your badge on throughout the event.','']
  }[kind];
  setField('welcome-eyebrow',copy[0]);setField('welcome-title',copy[1]);setField('welcome-copy',copy[2]);setField('welcome-fine',copy[3]);
  const ask=kind==='check-in';
  $('#checkin-button').hidden=!ask;$('#welcome-cancel').hidden=!ask;$('#welcome-done').hidden=ask;
  feedback(kind==='done'?'repeat':'success');
  show('welcome',ask?IDLE.welcome:IDLE.done);
}

async function checkIn(){
  if(state.busy||!state.current)return;
  const button=$('#checkin-button');state.busy=true;button.classList.add('loading');button.disabled=true;
  try{const x=await api('kiosk/check-in',{code:state.current.person.qrId});showWelcome(x.repeat?'done':'checked',x)}
  catch(err){if(err.status!==401)problem(err)}
  finally{state.busy=false;button.classList.remove('loading');button.disabled=false}
}

/* ---------- Camera ---------- */

function cameraMessage(text){const box=$('#camera-fallback');box.hidden=!text;if(text)$('#camera-fallback-text').textContent=text}
async function stopCamera(){
  const scanner=state.scanner;state.scanner=null;
  if(scanner){try{await scanner.stop()}catch{}try{scanner.clear()}catch{}}
}
async function startCamera(){
  clearTimeout(state.cameraRetry);
  await stopCamera();
  if(typeof Html5Qrcode!=='function'||!navigator.mediaDevices?.getUserMedia){cameraMessage('The camera is not available here. Please visit the registration desk.');return}
  $('#camera').classList.toggle('mirror',settings.camera==='user');
  cameraMessage('Starting the camera…');
  const scanner=new Html5Qrcode('camera-reader',{verbose:false,formatsToSupport:[Html5QrcodeSupportedFormats.QR_CODE],experimentalFeatures:{useBarCodeDetectorIfSupported:true}});
  state.scanner=scanner;
  try{
    await scanner.start({facingMode:settings.camera},{fps:12,aspectRatio:1},text=>void onCode(text),()=>{});
    cameraMessage('');
  }catch(error){
    if(state.scanner===scanner)state.scanner=null;
    try{scanner.clear()}catch{}
    const m=String(error);
    cameraMessage(/NotAllowed|Permission/i.test(m)?'Camera permission is needed. Staff: allow camera access for this site.':/NotReadable|Could not start/i.test(m)?'The camera is busy. It will retry in a moment.':'The camera could not start. It will retry in a moment.');
    state.cameraRetry=setTimeout(startCamera,10000);
  }
}

// USB and Bluetooth scanners type the code and press Enter; no field is focused.
const wedge={buffer:'',at:0};
document.addEventListener('keydown',e=>{
  if($('#staff-dialog').open||e.ctrlKey||e.altKey||e.metaKey)return;
  // Measure gaps by when each key was typed, not when it was handled: a busy
  // page delivers a fast scanner's keys late, and a code must not split in two.
  const now=e.timeStamp;
  if(now-wedge.at>300)wedge.buffer='';
  wedge.at=now;
  if(e.key==='Enter'){if(wedge.buffer.length>=6)void onCode(wedge.buffer);wedge.buffer='';e.preventDefault();return}
  if(e.key.length===1){wedge.buffer+=e.key;e.preventDefault()}
});

async function prepareBadge(person,qrUrl){
  const grid=qrUrl?Badge.qrGrid(await Badge.loadImage(qrUrl)):null;
  const preview=Badge.render(person,grid,300,false),canvas=$('#badge-preview');
  canvas.width=preview.width;canvas.height=preview.height;canvas.getContext('2d').drawImage(preview,0,0);
  state.badge=badgeImage(Badge.render(person,grid,badgeDpi(),badgeRotated()));
}

/* ---------- Printing ---------- */

function badgeImage(canvas){return {dataUrl:canvas.toDataURL('image/png'),width:canvas.width,height:canvas.height,rotate:badgeRotated()}}

// Hands the badge to the printer. Must run inside the tap that asked for it:
// Android only opens another app's intent from a user gesture.
function dispatchPrint(badge){
  if(settings.printer==='rawbt'){
    location.href=`intent:${badge.dataUrl}#Intent;scheme=rawbt;package=${RAWBT_PACKAGE};end;`;
    return;
  }
  const w=badge.rotate?LABEL_MM.h:LABEL_MM.w,h=badge.rotate?LABEL_MM.w:LABEL_MM.h;
  const sheet=[...document.styleSheets].find(s=>s.href&&s.href.endsWith('/kiosk.css'));
  if(sheet){if(state.pageRule>=0)sheet.deleteRule(state.pageRule);state.pageRule=sheet.insertRule(`@page{size:${w}mm ${h}mm;margin:0}`,sheet.cssRules.length)}
  const box=$('#print-sheet');box.replaceChildren();
  const img=new Image();img.src=badge.dataUrl;img.alt='';img.style.width=w+'mm';img.style.height=h+'mm';box.append(img);
  window.print();
}

// retry asks the server to print again a badge it already recorded, which it
// allows from this kiosk for a few minutes.
async function printBadge(retry,button){
  if(state.busy||!state.current||!state.badge)return;
  const app=printerMode()==='app';
  state.busy=true;button.classList.add('loading');button.disabled=true;clearIdle();
  try{
    // Check the printer first, so a printer that is down does not use up the attendee's print.
    if(app){const p=(await appBridge.status())?.printer;if(!p?.connected||!p.ready)throw printerError(p?.problem||'The badge printer is not connected',state.printRecorded)}
    await api('kiosk/print',{code:state.current.person.qrId,retry});
    state.printRecorded=true;
    if(app){
      show('printing');
      const r=await appBridge.print(state.badge);
      if(!r.ok)throw printerError(r.message,true);
    }else{dispatchPrint(state.badge);show('printing')}
    store.set('bcKioskPrinted',{day:state.today,count:printedToday()+1});
    feedback('success');
    setTimeout(()=>{
      if(state.screen!=='printing')return;
      $('#reprint-button').hidden=true;
      show('collect',IDLE.collect);
      setTimeout(()=>{if(state.screen==='collect')$('#reprint-button').hidden=false},5000);
    },app?1200:retry?2500:4200);
  }catch(err){
    if(err.status===401)return;
    if(err.code==='printer')return printerProblem(err.message,err.recorded);
    problem(err);
  }finally{state.busy=false;button.classList.remove('loading');button.disabled=false}
}
function printerProblem(message,recorded){
  setField('problem-eyebrow',recorded?"You're checked in":'Printer problem');
  setField('problem-title',recorded?"Your badge didn't print.":"The badge printer isn't ready.");
  setField('problem-copy',`${message}. Try printing again, or the registration desk will print your badge for you.`);
  setField('problem-reset',recorded?'Done':'Cancel');
  $('#problem-art').src='/static/kiosk/helpdesk.webp';
  $('#problem-retry').hidden=false;$('#problem-reset').classList.replace('btn-primary','btn-ghost');
  feedback('reject');
  show('problem',IDLE.printer);
  void appBridge?.status();
}
function printedToday(){const p=store.get('bcKioskPrinted',{});return p.day===state.today?p.count||0:0}

/* ---------- Feedback ---------- */

function unlockAudio(){try{const A=window.AudioContext||window.webkitAudioContext;if(!A)return null;const c=state.audio||(state.audio=new A());if(c.state==='suspended')void c.resume();return c}catch{return null}}
function feedback(kind){
  const sounds={tick:[[880,0,.05]],success:[[660,0,.1],[880,.11,.2]],repeat:[[520,0,.12],[520,.18,.12]],reject:[[330,0,.16],[247,.18,.26]]},notes=sounds[kind];
  try{const c=state.audio;if(c&&c.state==='running'&&notes){const t0=c.currentTime+.01;for(const [f,d,len] of notes){const o=c.createOscillator(),v=c.createGain(),at=t0+d;o.type='sine';o.frequency.setValueAtTime(f,at);v.gain.setValueAtTime(.0001,at);v.gain.exponentialRampToValueAtTime(.2,at+.012);v.gain.exponentialRampToValueAtTime(.0001,at+len);o.connect(v).connect(c.destination);o.start(at);o.stop(at+len+.02)}}}catch{}
}

/* ---------- Staff menu ---------- */

function openStaff(){
  const d=$('#staff-dialog');if(d.open)return;
  $('#unlock-form').hidden=false;$('#staff-panel').hidden=true;$('#unlock-error').textContent='';$('#unlock-passcode').value='';
  d.showModal();$('#unlock-passcode').focus();
  staffIdle();
}
function staffIdle(){clearTimeout(state.staffTimer);state.staffTimer=setTimeout(closeStaff,IDLE.staff*1000)}
function closeStaff(){clearTimeout(state.staffTimer);state.staffPass='';$('#unlock-passcode').value='';const d=$('#staff-dialog');if(d.open)d.close()}
function fillSettings(){
  const f=$('#settings-form');
  for(const key of ['printer','dpi','rotate','camera'])for(const input of f.elements[key])input.checked=String(settings[key])===input.value;
  // Inside the app, the app owns the printer; only the camera is set here.
  $$('[data-browser-printing]').forEach(el=>el.hidden=!!appBridge);
  if(appBridge){showPrinterStatus();void appBridge.status()}
  $('#staff-station').textContent=state.station;$('#staff-printed').textContent=printedToday();
}

let holdTimer=null;
const trigger=$('#staff-trigger');
trigger.addEventListener('pointerdown',e=>{e.preventDefault();trigger.classList.add('holding');holdTimer=setTimeout(()=>{trigger.classList.remove('holding');openStaff()},1600)});
for(const ev of ['pointerup','pointerleave','pointercancel'])trigger.addEventListener(ev,()=>{clearTimeout(holdTimer);trigger.classList.remove('holding')});
trigger.addEventListener('click',e=>e.preventDefault());

$('#unlock-form').addEventListener('submit',async e=>{
  e.preventDefault();staffIdle();
  const passcode=$('#unlock-passcode').value,button=e.submitter;
  button.disabled=true;$('#unlock-error').textContent='';
  try{await api('kiosk/unlock',{passcode});state.staffPass=passcode;$('#unlock-form').hidden=true;$('#staff-panel').hidden=false;fillSettings()}
  catch(err){$('#unlock-error').textContent=err.status===0?'No connection to the server.':err.status===401?'That passcode is not right.':err.message}
  finally{button.disabled=false;$('#unlock-passcode').value=''}
});
$('#settings-form').addEventListener('change',()=>{
  staffIdle();
  const f=new FormData($('#settings-form')),cameraBefore=settings.camera;
  settings={printer:f.get('printer'),dpi:Number(f.get('dpi')),rotate:Number(f.get('rotate')),camera:f.get('camera')};
  store.set('bcKioskSettings',settings);
  if(settings.camera!==cameraBefore)void startCamera();
});
$('#test-print').addEventListener('click',async()=>{
  staffIdle();
  try{
    await document.fonts.ready;
    const sample={name:'Dr. Lakshmi Narayanan Pillai',designation:'Principal Scientist',institution:'Rajiv Gandhi Centre for Biotechnology',reference:'BC4-TEST-0000'};
    const badge=badgeImage(Badge.render(sample,null,badgeDpi(),badgeRotated()));
    if(printerMode()!=='app')return dispatchPrint(badge);
    const box=$('#app-printer');box.hidden=false;box.classList.remove('problem');box.textContent='Printing a test badge…';
    const r=await appBridge.print(badge);
    box.classList.toggle('problem',!r.ok);box.textContent=r.ok?'Test badge sent to the printer.':`Test badge failed: ${r.message||'unknown printer error'}`;
  }catch{$('#unlock-error').textContent='Test badge could not be drawn.'}
});
$('#exit-kiosk').addEventListener('click',async e=>{
  const button=e.currentTarget;button.disabled=true;
  try{await api('kiosk/exit',{passcode:state.staffPass});location.replace('/ops')}
  catch(err){button.disabled=false;alertStaff(err.status===401?'Unlock again to exit.':err.message)}
});
function alertStaff(message){$('#staff-panel').hidden=true;$('#unlock-form').hidden=false;$('#unlock-error').textContent=message}
$$('[data-close]').forEach(b=>b.addEventListener('click',closeStaff));
$('#staff-dialog').addEventListener('cancel',e=>{e.preventDefault();closeStaff()});
$('#staff-dialog').addEventListener('pointerdown',staffIdle);

/* ---------- Wiring ---------- */

$$('[data-action="reset"]').forEach(b=>b.addEventListener('click',()=>reset(b.dataset.hold===undefined?HOLD_MS:Number(b.dataset.hold))));
$('#print-button').addEventListener('click',e=>void printBadge(false,e.currentTarget));
$('#reprint-button').addEventListener('click',e=>void printBadge(true,e.currentTarget));
$('#problem-retry').addEventListener('click',e=>void printBadge(state.printRecorded,e.currentTarget));
$('#checkin-button').addEventListener('click',()=>void checkIn());
$('#start-button').addEventListener('click',async e=>{
  const button=e.currentTarget;button.disabled=true;
  try{const x=await api('kiosk/start',{});state.csrf=x.csrf;state.paused=false;await boot()}
  catch(err){button.disabled=false;setField('paused-copy',err.message)}
});

// Any touch on a waiting screen restarts its countdown.
document.addEventListener('pointerdown',()=>{
  unlockAudio();goFullscreen();
  const idle=IDLE[state.screen];
  if(state.timer&&idle&&!['problem','done'].includes(state.screen))armIdle(idle);
},{capture:true});

function goFullscreen(){
  const el=document.documentElement;
  if(!document.fullscreenElement&&el.requestFullscreen&&!matchMedia('(display-mode: fullscreen)').matches)el.requestFullscreen({navigationUI:'hide'}).catch(()=>{});
}
async function keepAwake(){
  try{if('wakeLock' in navigator&&document.visibilityState==='visible'&&!state.wakeLock){state.wakeLock=await navigator.wakeLock.request('screen');state.wakeLock.addEventListener('release',()=>state.wakeLock=null)}}catch{}
}
document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible'){void keepAwake();if(!state.scanner&&!state.paused)void startCamera()}});
document.addEventListener('contextmenu',e=>e.preventDefault());
document.addEventListener('dragstart',e=>e.preventDefault());
document.addEventListener('gesturestart',e=>e.preventDefault());
// Keep the back gesture from leaving the kiosk.
history.pushState(null,'',location.href);
window.addEventListener('popstate',()=>history.pushState(null,'',location.href));

function setNet(online){$('#net-status').hidden=online}
window.addEventListener('online',()=>{setNet(true);void heartbeat()});
window.addEventListener('offline',()=>setNet(false));

function clock(){$('#clock').textContent=new Intl.DateTimeFormat('en-IN',{hour:'numeric',minute:'2-digit',hour12:true,timeZone:'Asia/Kolkata'}).format(new Date())}

async function heartbeat(){
  if(!state.kiosk)return;
  void appBridge?.status();
  try{
    const me=await api('me');setNet(true);
    if(!me.kiosk){state.kiosk=false;return pause('staff')}
    if(me.today!==state.today){state.today=me.today;state.days=me.days;dayLabel()}
    resume();
  }catch(err){if(err.status===0)setNet(false)}
}
function dayLabel(){const i=state.days.findIndex(d=>d.ID===state.today),d=state.days[i];$('#day-label').textContent=d?`Day ${i+1} · ${d.Label.split('·')[1]?.trim()||d.Label}`:''}

async function boot(){
  void appBridge?.status();
  await Badge.fontsReady();
  let me;
  try{me=await api('me')}catch(err){document.body.classList.remove('booting');if(err.status===0){setNet(false);pause('expired');setTimeout(boot,10000)}else pause('setup');return}
  Object.assign(state,{csrf:me.csrf,station:me.station,today:me.today,days:me.days});
  dayLabel();
  document.body.classList.remove('booting');
  state.kiosk=me.kiosk;
  if(!me.kiosk)return pause('staff');
  state.paused=false;
  show('scan');
  void keepAwake();
  void startCamera();
}
clock();setInterval(clock,15000);
setInterval(()=>void heartbeat(),60000);
void boot();
