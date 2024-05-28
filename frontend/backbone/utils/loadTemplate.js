import _ from 'underscore';
import { ipc } from '../../src/utils/ipcRenderer.js';
import * as templateHelpers from './templateHelpers';

const templateCache = {};

let helpers = {};

export default function loadTemplate(templateFile, callback) {
  if (!templateFile) {
    throw new Error('Please provide a path to the template.');
  }

  _.templateSettings.variable = 'ob';

  let template = templateCache[templateFile];

  if (!template) {
    const file = ipc.sendSync('controller.system.readTemplateFileSync', templateFile);
    template = _.template(file);
    templateCache[templateFile] = template;
  }

  const sendBackTmpl = () => {
    const wrappedTmpl = (context) => template({ ...templateHelpers, ...helpers, ...(context || {}) });
    callback(wrappedTmpl);
  };

  if (!helpers.formErrorTmpl) {
    helpers.formErrorTmpl = 'its coming'; // hack to avoid infinite recursion

    loadTemplate('spinner.svg', (t) => {
      helpers.spinner = t;
    });

    loadTemplate('processingButton.html', (t) => {
      helpers.processingButton = t;
    });

    loadTemplate('formError.html', (t) => {
      helpers.formErrorTmpl = t;
      sendBackTmpl();
    });
  } else {
    sendBackTmpl();
  }
}
