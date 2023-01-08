import { readFileSync, writeFileSync } from 'fs';

const indexIn = `${__dirname}/../index.html`;
const indexOut = `${__dirname}/../.tmp/index.html`;
const bsClientScriptPath = `${__dirname}/../js/lib/bs-client.js`;
let indexHtml;
let bsClientScript;

try {
  indexHtml = readFileSync(indexIn);
} catch (e) {
  throw new Error(`Unable to read ${indexIn}. ${e}`);
}

try {
  bsClientScript = readFileSync(bsClientScriptPath);
} catch (e) {
  throw new Error(`Unable to read ${bsClientScript}. ${e}`);
}

try {
  const content = String(indexHtml).replace('<!-- NODE_ENV -->', process.env.NODE_ENV)
    .replace('<!-- BROWSER_SYNC_PLACEHOLDER (DO NOT REMOVE OR ALTER!) -->', String(bsClientScript));
  writeFileSync(indexOut, content);
} catch (e) {
  throw new Error(`Unable to write to ${indexOut}. ${e}`);
}
