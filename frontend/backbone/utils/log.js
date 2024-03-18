import { ipc } from '../../src/utils/ipcRenderer.js';

export function printLog(args) {
  ipc.send('controller.system.printLog', args);
}