import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from '/home/mshiyaf/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
const root=process.cwd(), out=path.join(root,'outputs/store-assets-styled');
const data=async(p,type='image/png')=>`data:${type};base64,${(await fs.readFile(path.join(root,p))).toString('base64')}`;
const background=await data('outputs/store-assets-styled/campaign-background.png');
const manrope=await data('mobile/assets/Manrope-Bold.ttf','font/ttf');
const dm=await data('mobile/assets/DMSans-Regular.ttf','font/ttf');
const logo=await data('mobile/assets/images/bio-connect-logo.png');
const screens=[
 ['01-home','Your event.\nAt a glance.','Meet your Bio Connect companion.'],
 ['02-speakers','Meet the minds\nbehind the science.','Explore speakers and their organisations.'],
 ['03-guide','Everything you need.\nOne event guide.','Venue, exhibitors, registration and more.'],
 ['04-venue','Find your way\nto Bio Connect.','Venue information and directions.'],
 ['05-activities','Discover. Meet.\nConnect.','Explore programme highlights.'],
];
const fonts=`@font-face{font-family:Manrope;src:url('${manrope}')}@font-face{font-family:DM;src:url('${dm}')}*{box-sizing:border-box}body{margin:0;font-family:DM;color:#0b3329}`;
const browser=await chromium.launch({headless:true});
for(const [name,title,subtitle] of screens){
 const image=await data(`outputs/store-assets/phone-screenshots/${name}.png`);
 const page=await browser.newPage({viewport:{width:1080,height:1920},deviceScaleFactor:1});
 await page.setContent(`<style>${fonts}body{width:1080px;height:1920px;background:#f3f1e9 url('${background}') center/cover}.brand{position:absolute;top:60px;left:72px;letter-spacing:3px;font-size:22px}.copy{position:absolute;top:135px;left:72px}h1{font-family:Manrope;font-size:66px;line-height:1.13;letter-spacing:-2px;margin:0;white-space:pre-line}p{font-size:27px;color:#466357;margin-top:24px}.phone{position:absolute;left:238px;top:470px;width:604px;height:1208px;border:12px solid #172821;border-radius:64px;background:#172821;box-shadow:0 48px 85px #15352733,0 8px 15px #15352722;overflow:hidden}.phone img{width:100%;height:100%;display:block;object-fit:fill;border-radius:49px}.footer{position:absolute;bottom:68px;left:72px;font-size:21px;letter-spacing:2px;color:#466357}</style><div class="brand">BIO CONNECT 4.0</div><div class="copy"><h1>${title}</h1><p>${subtitle}</p></div><div class="phone"><img src="${image}"></div><div class="footer">CONNECTING SCIENCE TO BUSINESS</div>`);
 await page.evaluate(()=>document.fonts.ready);await page.screenshot({path:path.join(out,`${name}-1080x1920.png`)});await page.close();
}
const screen=await data('outputs/store-assets/phone-screenshots/01-home.png');
const page=await browser.newPage({viewport:{width:1024,height:500},deviceScaleFactor:1});
await page.setContent(`<style>${fonts}body{width:1024px;height:500px;background:#f3f1e9 url('${background}') center 76%/cover}body:before{content:'';position:absolute;inset:0;background:linear-gradient(90deg,#f3f1e9 0%,#f3f1e9 47%,#f3f1e9ee 55%,transparent 75%)}.logo{position:absolute;left:56px;top:55px;width:385px}h1{position:absolute;left:56px;top:174px;font-family:Manrope;font-size:40px;line-height:1.15;letter-spacing:-1px;margin:0}p{position:absolute;left:56px;top:288px;font-size:18px;line-height:1.6;color:#466357}.edition{position:absolute;left:56px;bottom:48px;letter-spacing:3px;font-size:15px}.phone{position:absolute;left:656px;top:48px;width:202px;height:404px;border:5px solid #172821;border-radius:24px;overflow:hidden;transform:rotate(8deg);box-shadow:15px 24px 38px #15352733;background:#172821}.phone img{width:100%;height:100%;display:block;border-radius:18px}</style><img class="logo" src="${logo}"><h1>Your life sciences<br>event companion.</h1><p>Discover speakers. Explore the programme.<br>Plan your Bio Connect experience.</p><div class="edition">BIO CONNECT 4.0</div><div class="phone"><img src="${screen}"></div>`);
await page.evaluate(()=>document.fonts.ready);await page.screenshot({path:path.join(out,'feature-graphic-1024x500.png')});await browser.close();
console.log('Rendered five screenshot cards and feature graphic');
