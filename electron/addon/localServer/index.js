const { app: electronApp, ipcMain } = require('electron');
const _ = require('underscore');
const { EOL, platform } = require('os');
const { Events } = require('backbone');
const path = require('path');
const fs = require('fs');
const childProcess = require('child_process');
const UtilsPs = require('ee-core/ps');

const Log = require('ee-core/log');
const Conf = require('ee-core/config');

function s4() {
  return (((1 + Math.random()) * 0x10000) | 0).toString(16).substring(1);
}

function guid(prefix = '') {
  return `${prefix}${s4()}${s4()}-${s4()}-${s4()}-${s4()}-${s4()}${s4()}${s4()}`;
}

class LocalServerAddon {
  constructor() {
  }

  create () {
    Log.info('[addon:localServer] load');

    _.extend(this, Events);
    this.serverPath = path.join(UtilsPs.getExtraResourcesDir(), 'mobazha');
    this.serverFilename = process.platform === 'darwin' || process.platform === 'linux' ? 'mobazhad' : 'mobazha.exe';
    // If not bundled app, don't use local server.
    this._isBundledApp = fs.existsSync(path.join(this.serverPath, this.serverFilename));
    if (!this._isBundledApp) {
      return;
    }

    global.isBundledApp = this._isBundledApp;
    global.authCookie = guid();

    this._isRunning = false;
    this._isStopping = false;
    this._debugLog = '';
    this._lastStartCommandLineArgs = [];
    this.startAfterStop = (...args) => this.start(...args);

    ipcMain.on('server-shutdown-fail', (e, data = {}) => {
      if (this.isStopping && this.serverSubProcess) {
        const reasonInsert = data.reason ? ` (${data.reason})` : '';
        const logMsg = `The server shutdown via api request failed${reasonInsert}. `
          + 'Will forcibly shutdown.';

        Log.info(logMsg);
        this._forceKill();
      }
    });

    // some cleanup when our app is exiting
    process.on('exit', () => {
      this.stop();
    });
  }

  get isEnabled() {
    return this._isBundledApp;
  }

  get isRunning() {
    return this._isRunning;
  }

  get isStopping() {
    return this._isStopping;
  }

  /**
   * The command line args that the server was last started with. Will default to an
   * empty array if the server has never been started this session. This only applies
   * to the server being started via start(), not the server being started in status
   * mode (via getServerStatus()).
   */
  get lastStartCommandLineArgs() {
    return this._lastStartCommandLineArgs;
  }

  start(commandLineArgs = []) {
    if (this.isStopping) {
      this.serverSubProcess.once('exit', () => this.startAfterStop(commandLineArgs));
      const debugInfo = 'Attempt to start server while an existing one'
        + ' is the process of shutting down. Will start after shut down is complete.';
      Log.info(debugInfo);
      return;
    }

    if (this.isRunning) {
      if (_.isEqual(commandLineArgs, this._lastStartCommandLineArgs)) {
        return;
      }

      throw new Error('A server is already running with different command line options. Please '
        + 'stop that server before starting a new one.');
    }

    this._isRunning = true;
    let serverStartArgs = ['start', ...commandLineArgs];

    // wire in our auth cookie
    if (global.authCookie) {
      serverStartArgs = serverStartArgs.concat(['--apicookie', global.authCookie]);
    }

    Log.info(`Starting local server via '${serverStartArgs.join(' ')}'.`);
    console.log(`Starting local server via '${serverStartArgs.join(' ')}'.`);

    this._lastStartCommandLineArgs = commandLineArgs;
    this.serverSubProcess = childProcess.spawn(
      path.join(this.serverPath, this.serverFilename),
      serverStartArgs,
      {
        detach: false,
        cwd: this.serverPath,
      },
    );

    this.serverSubProcess.stdout.once('data', () => {
      this.trigger('start');
    });
    this.serverSubProcess.stdout.on('data', (buf) => this.obServerLog(`${buf}`));

    this.serverSubProcess.on('error', (err) => {
      const errOutput = `The local server child process has an error: ${err}`;

      Log.error('[addon:localServer] ', errOutput);
    });

    this.serverSubProcess.stderr.on('data', (buf) => {
      Log.error('[addon:localServer] ', String(buf));

      this.obServerLog(`${buf}`, 'STDERR');
    });

    this.serverSubProcess.on('exit', (code, signal) => {
      let logMsg;

      if (code !== null) {
        logMsg = `Server exited with code: ${code}`;
      } else {
        logMsg = `Server exited at request of signal: ${signal}.`;
      }

      console.log(logMsg);
      Log.info(logMsg, 'EXIT');
      this._isRunning = false;
      this.lastCloseCode = code;
      this.trigger('exit', { code });
    });

    this.serverSubProcess.unref();
  }

  _forceKill() {
    if (platform() !== 'win32') {
      throw new Error('For non windows OSs, use childProcess.kill and pass in a signal.');
    }

    if (!this.isStopping) {
      throw new Error('A force kill should only be attempted if you tried stopping via this.stop '
        + 'and it failed.');
    }

    if (this.serverSubProcess) {
      Log.info('Forcibly shutting down the server via taskkill.');
      childProcess.spawn('taskkill', ['/pid', this.serverSubProcess.pid, '/f', '/t']);
    }
  }

  stop() {
    if (!this.isRunning) return;

    if (this.isStopping) {
      this.serverSubProcess.removeListener('exit', this.startAfterStop);
      return;
    }

    this._isStopping = true;
    this.serverSubProcess.once('exit', () => (this._isStopping = false));

    Log.info('Shutting down server');
    console.log('Shutting down server');

    if (platform() === 'darwin' || platform() === 'linux') {
      this.serverSubProcess.kill('SIGINT');
    } else {
      const mw = electronApp.mainWindow;

      if (mw) {
        mw.webContents.send('server-shutdown');
      } else {
        this._forceKill();
      }
    }
  }

  getServerStatus(commandLineArgs = []) {
    Log.info('Starting local server in status mode.');
    console.log('Starting local server in status mode.');

    const subProcess = childProcess.spawn(
      path.join(this.serverPath, this.serverFilename),
      ['status', ...commandLineArgs],

      {
        detach: false,
        cwd: this.serverPath,
      },
    );

    subProcess.stdout.on('data', (buf) => this.obServerStatusLog(`${buf}`));

    subProcess.on('error', (err) => {
      const errOutput = `Starting local server in status mode produced an error: ${err}`;

      Log.error('[addon:localServer] ', errOutput);
    });

    subProcess.stderr.on('data', (buf) => {
      Log.error('[addon:localServer] ', `[OB-SERVER-STATUS] ${String(buf)}`);

      this.obServerStatusLog(`${buf}`, 'STDERR', true);
    });

    subProcess.on('exit', (code, signal) => {
      let logMsg;

      if (code !== null) {
        let encrypted = false;
        let torAvailable = false;
        logMsg = `Local server status mode exited with code: ${code}`;

        if (code === 1) {
          this.trigger('getServerStatusFail', { pid: subProcess.pid });
        } else if (code === 30) {
          encrypted = true;
        } else if (code === 31) {
          encrypted = true;
          torAvailable = true;
        } else if (code === 21 || code === 11) {
          torAvailable = true;
        }

        this.trigger('getServerStatusSuccess', {
          pid: subProcess.pid,
          torAvailable,
          encrypted,
        });
      } else {
        logMsg = `Local server status mode exited at request of signal: ${signal}.`;
        this.trigger('getServerStatusFail', { pid: subProcess.pid });
      }

      console.log(logMsg);
      Log.info(logMsg, 'EXIT');
    });

    subProcess.unref();

    return subProcess.pid;
  }

  get debugLog() {
    return this._debugLog;
  }

  _log(msg, type = 'LOCAL-SERVER') {
    const newLog = `[${type}] ${msg}${msg.endsWith(EOL) ? '' : EOL}`;
    this._debugLog += newLog;
    this.trigger('log', this, newLog);
  }

  log(msg) {
    if (typeof msg !== 'string') {
      throw new Error('Please provide a message.');
    }

    if (!msg) return;
    this._log(msg);
  }

  obServerLog(msg, type = 'STDOUT', serverType = '[OB-SERVER]') {
    if (typeof msg !== 'string') {
      throw new Error('Please provide a message.');
    }

    if (!msg) return;
    console.log(msg);

    const msgPreface = type ? `[${type}] ` : '';
    msg.split(EOL).forEach((splitMsg) => this._log(`${msgPreface}${splitMsg}`, serverType));
  }

  obServerStatusLog(msg, type = 'STDOUT') {
    this.obServerLog(msg, type, '[OB-SERVER-STATUS]');
  }
}

LocalServerAddon.toString = () => '[class LocalServerAddon]';
module.exports = LocalServerAddon;