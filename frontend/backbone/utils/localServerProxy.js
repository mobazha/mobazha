
import _ from 'underscore';
import { Events } from 'backbone';
import { ipc } from '@/utils/ipcRenderer.js';

export default class {

  constructor() {
    this.localServerRoute = {};
    ['isRunning', 'isStopping', 'lastStartCommandLineArgs', 'start', 'stop', 'getServerStatus'].forEach(route => {
      this.localServerRoute[route] = `controller.localServer.${route}`;
    })

    _.extend(this, Events);

    this.mapIPCEvents();
  }

  mapIPCEvents() {
    const routePrefix = 'controller.localServer.events';

    ipc.on(`${routePrefix}.start`, () => {
      this.trigger('start');
    });

    ipc.on(`${routePrefix}.getServerStatusFail`, (event, data) => {
      this.trigger('getServerStatusFail', data);
    });

    ipc.on(`${routePrefix}.getServerStatusSuccess`, (event, data) => {
      this.trigger('getServerStatusSuccess', data);
    });
  }

  get isRunning() {
    return ipc.sendSync(this.localServerRoute.isRunning);
  }

  get isStopping() {
    return ipc.sendSync(this.localServerRoute.isStopping);
  }

  /**
   * The command line args that the server was last started with. Will default to an
   * empty array if the server has never been started this session. This only applies
   * to the server being started via start(), not the server being started in status
   * mode (via getServerStatus()).
   */
  get lastStartCommandLineArgs() {
    return ipc.sendSync(this.localServerRoute.lastStartCommandLineArgs);
  }

  start(commandLineArgs = []) {
    ipc.send(this.localServerRoute.start, commandLineArgs);
  }

  stop() {
    ipc.send(this.localServerRoute.stop);
  }

  getServerStatus() {
    return ipc.sendSync(this.localServerRoute.getServerStatus);
  }
}