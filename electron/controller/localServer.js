'use strict';

const { Controller } = require('ee-core');
const Addon = require('ee-core/addon');

/**
 * LocalServer
 * @class
 */
class LocalServerController extends Controller {

  constructor(ctx) {
    super(ctx);

    this.localServer = Addon.get('localServer');

    if (this.localServer.isEnabled) {
      this.initEvents();
    }
  }

  initEvents() {
    this.mainWindow = this.app.electron.mainWindow;

    const routePrefix = 'controller.localServer.events';
    this.localServer.on('start', () => {
      this.mainWindow.webContents.send(`${routePrefix}.start`);
    });

    this.localServer.on('getServerStatusSuccess', (data) => {
      this.mainWindow.webContents.send(`${routePrefix}.getServerStatusSuccess`, data);
    });

    this.localServer.on('getServerStatusFail', (data) => {
      this.mainWindow.webContents.send(`${routePrefix}.getServerStatusFail`, data);
    });
  }

  isEnabled(args, event) {
    return this.localServer.isEnabled;
  }

  isRunning(args, event) {
    return this.localServer.isRunning;
  }

  isStopping(args, event) {
    return this.localServer.isStopping;
  }

  lastStartCommandLineArgs(args, event) {
    return this.localServer.lastStartCommandLineArgs;
  }

  start(commandLineArgs = [], event) {
    this.localServer.start(commandLineArgs);
  }

  stop(args, event) {
    this.localServer.stop();
  }

  getServerStatus(commandLineArgs = [], event) {
    return this.localServer.getServerStatus(commandLineArgs);
  }
}

LocalServerController.toString = () => '[class LocalServerController]';
module.exports = LocalServerController;  