'use strict';

const { session } = require('electron');
const { Controller } = require('ee-core');
const Log = require('ee-core/log');
const Addon = require('ee-core/addon');

/**
 * MainWindow
 * @class
 */
class MainWindowController extends Controller {

  constructor(ctx) {
    super(ctx);

    this.mainWindow = this.app.electron.mainWindow;
    this.closeConfirmed = false;
  }

  async doMainWindowAction (action, event) {
    if (typeof this.mainWindow[action] === 'function') {
      if (action == 'setFullScreen') {
        const isFullScreen = this.mainWindow.isFullScreen();
        this.mainWindow.setFullScreen(!isFullScreen);
      } else {
        this.mainWindow[action]();
      }
    }
    Log.info('do action: ', action);
  }

  async setProxy(args, event) {
    if (!args.id) {
      throw new Error('Please provide an id that will be passed back with the "proxy-set" '
        + 'event.');
    }

    this.mainWindow.webContents.session.setProxy({
      proxyRules: args.socks5Setting,
      proxyBypassRules: '<local>',
    }).then(() => this.mainWindow.webContents.send('proxy-set', args.id));
  }

  // If appropriate, add in Basic Auth headers to each request. If connecting to
  // the built-in server, we'll add in the auth token.
  async setActiveServer(server, event) {
    const filter = {
      urls: [`${server.httpUrl}*`, `${server.socketUrl}*`],
    };

    session.defaultSession.webRequest.onBeforeSendHeaders(filter, (details, callback) => {
      if (server.authenticate) {
        const un = server.username;
        const pw = server.password;

        details.requestHeaders.Authorization = `Basic ${new Buffer(`${un}:${pw}`).toString('base64')}`;
      }

      if (global.authCookie && server.builtIn) {
        details.requestHeaders.Cookie = `OpenBazaar_Auth_Cookie=${global.authCookie}`;
      }

      callback({ cancel: false, requestHeaders: details.requestHeaders });
    });
  };
  
  async confirmClose(args, event) {
    this.closeConfirmed = true;
    
    if (this.mainWindow) this.mainWindow.close();
  }

  async setBadgeCount(count, event) {
    // setBadgeCount is only available on certain environements:
    // https://github.com/electron/electron/blob/master/docs/api/app.md#appsetbadgecountcount-linux-macos
    try {
      this.app.setBadgeCount(count);
    } catch (err) {
      // pass
      console.log(err);
    }
  }

  async installUpdate(args, event) {
    Addon.get('autoUpdater').installUpdate();
  }

  async checkForUpdate(args, event) {
    Addon.get('autoUpdater').checkUpdate();
  }
}

MainWindowController.toString = () => '[class MainWindowController]';
module.exports = MainWindowController;  