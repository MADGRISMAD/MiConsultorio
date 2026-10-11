import { chromium } from 'playwright';
const S='/private/tmp/claude-501/-Users-manuelsabino-Desktop-MiColmena/d1d2bc58-b358-467e-a351-5f091feb2e41/scratchpad/';
const b=await chromium.launch(); const p=await (await b.newContext({viewport:{width:1280,height:300},reducedMotion:'reduce'})).newPage();
await p.goto('http://localhost:5180/',{waitUntil:'networkidle'}); await p.waitForTimeout(800);
await p.screenshot({path:S+'caresia-nav.png'}); await b.close();
