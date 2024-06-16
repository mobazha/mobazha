import _ from 'underscore';
import { Events } from 'backbone';

class IpcHttpCustom {
  constructor() {
    _.extend(this, Events);
  }

  send (channel, args) {
    switch (channel) {
      case 'controller.mainwindow.doMainWindowAction':
      case 'controller.mainwindow.checkForUpdate':
      case 'controller.mainwindow.installUpdate':
      case 'controller.mainwindow.setBadgeCount':
        break;
      
      case 'renderer-cleared-local-server-events':
      case 'server-shutdown-fail':
      case 'server-connect-log':
      case 'server-connect-ready':
      case 'controller.system.printLog':
        break;

      case 'controller.system.openExternal':
      

      case 'controller.mainwindow.setProxy': {
        this.trigger('proxy-set', args, args?.id);
        break;
      }

      case 'controller.mainwindow.setActiveServer':

      case 'controller.system.writeToClipboard':
        navigator.clipboard.writeText(args);
        break;
    
      default:
        break;
    }
  }

  synchronousRequest(url) {
    const xhr = new XMLHttpRequest();
    xhr.open('GET', url, false);
    xhr.send(null);
    if (xhr.status === 200) {
       return xhr.responseText;
    } else {
       throw new Error('Request failed: ' + xhr.statusText);
    }
  }

  sendSync (channel, args) {
    switch (channel) {
      case 'controller.system.getGlobal': {
        switch (args) {
          case 'externalRoute':
            return undefined;
          case 'isBundledApp':
            return false;
          case 'localServer':
            return undefined;
          default:
            break;
        }
        break;
      }
      case 'controller.system.getPlatform':
        return undefined;

      // 'isRunning', 'isStopping', 'lastStartCommandLineArgs', 'start', 'stop', 'getServerStatus'
      case 'controller.localServer.isRunning':
      case 'controller.localServer.isStopping':
      case 'controller.localServer.lastStartCommandLineArgs':
      case 'controller.localServer.start':
      case 'controller.localServer.stop':
      case 'controller.localServer.getServerStatus':
        return undefined;

      case 'controller.system.readTemplateFileSync':
        return this.synchronousRequest("/templates/"+args);
      case 'controller.system.getlanguageFileContent':
        return JSON.parse(this.synchronousRequest("/languages/"+args));
      default:
        break;
    }
  }

  removeListener (channel, listener) {
    this.off(channel, listener);
  }
}

export const ipcHttpCustom = new IpcHttpCustom();
