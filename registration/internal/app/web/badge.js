'use strict';
// Badge renderer shared by the staffed desk (/ops) and the self-service kiosk,
// so a badge looks the same whichever station printed it. It draws the
// 76.2 × 50.8 mm label as a canvas at the printer's resolution.
const Badge=(()=>{
const LABEL_MM={w:76.2,h:50.8};
// Most thermal label printers are 203 dpi; drawing at the printer's own
// resolution keeps every QR module a whole number of dots.
const DEFAULT_DPI=203;

function loadImage(src){return new Promise((ok,no)=>{const img=new Image();img.onload=()=>ok(img);img.onerror=no;img.src=src})}

// Finds the QR's module grid in the server PNG so it can be redrawn with whole
// printer dots per module: a thermal head cannot print a fraction of a dot.
function qrGrid(img){
  const c=document.createElement('canvas');c.width=img.naturalWidth;c.height=img.naturalHeight;
  const g=c.getContext('2d',{willReadFrequently:true});g.drawImage(img,0,0);
  const {data,width,height}=g.getImageData(0,0,c.width,c.height);
  const dark=(x,y)=>data[(y*width+x)*4]<128;
  let left=width,top=height,right=-1,bottom=-1;
  for(let y=0;y<height;y++)for(let x=0;x<width;x++)if(dark(x,y)){if(x<left)left=x;if(x>right)right=x;if(y<top)top=y;if(y>bottom)bottom=y}
  if(right<0)throw new Error('empty QR');
  let run=0;while(left+run<=right&&dark(left+run,top))run++;
  const pixel=run/7,size=right-left+1;
  return {img,left,top,size,modules:Math.round(size/pixel)};
}

function wrapText(g,text,width,maxLines){
  const words=String(text||'').split(/\s+/).filter(Boolean),lines=[];let line='';
  for(const word of words){
    const next=line?line+' '+word:word;
    if(g.measureText(next).width<=width){line=next;continue}
    if(line)lines.push(line);
    line=word;
  }
  if(line)lines.push(line);
  const fits=lines.length<=maxLines&&lines.every(l=>g.measureText(l).width<=width);
  return {lines,fits};
}
function clampLines(g,lines,width,maxLines){
  const out=lines.slice(0,maxLines);
  if(lines.length>maxLines||out.some(l=>g.measureText(l).width>width)){
    let last=out[out.length-1]+(lines.length>maxLines?'…':'');
    while(last.length>1&&g.measureText(last).width>width)last=last.slice(0,-2)+'…';
    out[out.length-1]=last;
  }
  return out;
}

// Draws the 76.2 × 50.8 mm badge at the printer's resolution: name, role and
// organisation above the QR, pass number below, matching the staffed desk badge.
function renderBadge(person,grid,dpi,rotate){
  const px=mm=>mm/25.4*dpi,pt=v=>v/72*dpi;
  const W=Math.round(px(LABEL_MM.w)),H=Math.round(px(LABEL_MM.h));
  const c=document.createElement('canvas');c.width=W;c.height=H;
  const g=c.getContext('2d');
  g.fillStyle='#fff';g.fillRect(0,0,W,H);g.fillStyle='#000';g.textAlign='center';g.textBaseline='alphabetic';
  const padX=px(3.8),padY=px(2.8),maxW=W-padX*2;
  const refSize=pt(8),refGap=px(.9);
  // QR: the largest whole-dot module size up to the desk badge's 19.5 mm.
  const modules=grid?grid.modules:33;
  let dot=Math.max(2,Math.floor(px(19.5)/modules));
  const textMin=pt(11)*1.9+pt(8);
  while(dot>2&&H-padY*2-refSize-refGap-modules*dot-px(1.4)<textMin)dot--;
  const qrSize=modules*dot,qrTop=H-padY-refSize-refGap-qrSize;
  const textBottom=qrTop-px(1.4);

  // A food pass (foodOnly in model.go) leads its role line with FOOD ONLY, so
  // volunteers can tell it apart at a glance.
  const food=person.categoryId==='food',role=food?['FOOD ONLY',person.designation].filter(Boolean).join(' · '):person.designation||person.category||'Delegate';
  const name=String(person.name||'').toUpperCase(),designation=String(role).toUpperCase(),institution=String(person.institution||'').toUpperCase();
  const fontName=s=>`800 ${s}px Manrope, Arial, sans-serif`,fontRole=s=>`800 ${s}px Manrope, Arial, sans-serif`,fontOrg=s=>`500 ${s}px "DM Sans", Arial, sans-serif`;
  let layout=null;
  for(let scale=1;scale>=.55;scale-=.05){
    const nameSize=pt(18)*scale,roleSize=pt(9)*Math.max(scale,.8),orgSize=pt(8)*Math.max(scale,.8);
    g.font=fontName(nameSize);const n=wrapText(g,name,maxW,2);
    if(!n.fits&&scale>.56)continue;
    g.font=fontRole(roleSize);const role=clampLines(g,[designation],maxW,1);
    g.font=fontOrg(orgSize);const org=institution?clampLines(g,wrapText(g,institution,maxW,2).lines,maxW,2):[];
    g.font=fontName(nameSize);const nameLines=clampLines(g,n.lines,maxW,2);
    const height=nameLines.length*nameSize*.98+px(1.1)+roleSize*1.1+(org.length?px(.7)+org.length*orgSize*1.12:0);
    layout={nameSize,roleSize,orgSize,nameLines,role,org};
    if(padY+height<=textBottom)break;
  }
  let y=padY;
  g.font=fontName(layout.nameSize);
  for(const line of layout.nameLines){y+=layout.nameSize*.92;g.fillText(line,W/2,y);y+=layout.nameSize*.06}
  y+=px(1.1)+layout.roleSize*.9;g.font=fontRole(layout.roleSize);g.fillText(layout.role[0],W/2,y);
  if(layout.org.length){y+=px(.7);g.font=fontOrg(layout.orgSize);for(const line of layout.org){y+=layout.orgSize*1.02;g.fillText(line,W/2,y)}}

  const qrLeft=Math.round((W-qrSize)/2),qrY=Math.round(qrTop);
  if(grid){g.imageSmoothingEnabled=false;g.drawImage(grid.img,grid.left,grid.top,grid.size,grid.size,qrLeft,qrY,qrSize,qrSize)}
  else{g.lineWidth=Math.max(2,dot);g.setLineDash([dot*3,dot*2]);g.strokeRect(qrLeft,qrY,qrSize,qrSize);g.setLineDash([]);g.font=`800 ${pt(10)}px Manrope, Arial, sans-serif`;g.fillText('TEST',W/2,qrY+qrSize/2+pt(4))}
  g.font=`700 ${refSize}px ui-monospace, "Roboto Mono", monospace`;g.fillText(person.reference||'',W/2,H-padY);

  if(!rotate)return c;
  const r=document.createElement('canvas');r.width=H;r.height=W;
  const rg=r.getContext('2d');rg.translate(H,0);rg.rotate(Math.PI/2);rg.drawImage(c,0,0);
  return r;
}

// Canvas does not trigger webfont loading, so wait for the badge fonts first.
function fontsReady(){return Promise.all([document.fonts.load('800 40px Manrope'),document.fonts.load('500 20px "DM Sans"')]).catch(()=>{})}

// Renders a pass's badge as a PNG data URL, ready for any printer path.
async function png(person,qrUrl,{dpi=DEFAULT_DPI,rotate=false}={}){
  await fontsReady();
  const grid=qrUrl?qrGrid(await loadImage(qrUrl)):null;
  return renderBadge(person,grid,dpi,rotate).toDataURL('image/png');
}

return {LABEL_MM,DEFAULT_DPI,loadImage,qrGrid,render:renderBadge,fontsReady,png};
})();
