import sifter from 'sifter';
import microplugin from 'microplugin';
import $ from "jquery";
import bigNumber from 'bignumber.js';
import moment from 'moment';

import app from '../backbone/app';
import Polyglot from '../backbone/utils/Polyglot';
import LocalSettings from '../backbone/models/LocalSettings';
import { getTranslationLangByCode } from '../backbone/data/languages';

import ServerConfigs from '../backbone/collections/ServerConfigs';

import { ipc } from './utils/ipcRenderer.js';

window.jQuery = window.$ = $;
window.Sifter = sifter;
window.MicroPlugin = microplugin;


// Will allow us to handle numbers with greater than 20 decimals places. Probably
// unlikely this will be needed, but just in case.
bigNumber.config({ DECIMAL_PLACES: 50 });

app.localSettings = new LocalSettings({ id: 1 });
app.localSettings.fetch().fail(() => app.localSettings.save());

// initialize language functionality
function getValidLanguage(lang) {
  if (getTranslationLangByCode(lang)) {
    return lang;
  }

  return 'en_US';
}

const initialLang = getValidLanguage(app.localSettings.get('language'));
app.localSettings.set('language', initialLang);
moment.locale(initialLang);
app.polyglot = new Polyglot();
const langContent = ipc.sendSync('controller.system.getlanguageFileContent', `${initialLang}.json`);
app.polyglot.extend(langContent);

// Instantiating our Server Configs collection now since the page nav
// utilizes it. We'll fetch it later on.
app.serverConfigs = new ServerConfigs();