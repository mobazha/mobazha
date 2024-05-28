/* eslint-disable class-methods-use-this */
import { Collection } from 'backbone';
import { ipc } from '../../src/utils/ipcRenderer.js';
import app from '../app';
import LocalStorageSync from '../utils/lib/backboneLocalStorage';
import ServerConfig from '../models/ServerConfig';

export default class extends Collection {
  localStorage() {
    return new LocalStorageSync('__serverConfigs');
  }

  sync(...args) {
    return LocalStorageSync.sync.apply(this, args);
  }

  model(attrs, options) {
    return function(attrs, options) {
      return new ServerConfig(attrs, options);
    }(attrs, options);
  }

  constructor(models, options) {
    super(models, options);
    this._activeId = localStorage.activeServerConfig;
    this.on('sync', () => this.bindActiveServerChangeHandler());
  }

  /**
   * The "active" server is the server we are currently connected to or if we're not
   * connected to any server, it's the last server we were connected to. When the app is
   * re-started, a connection will automatically be attempted to this server.
   */
  get activeServer() {
    return this.get(this._activeId);
  }

  set activeServer(md) {
    if (!(md instanceof ServerConfig)) {
      throw new Error('Please provide a model as a ServerConfig instance.');
    }

    if (this.models.indexOf(md) === -1) {
      throw new Error('The provided model is not in this collection and must be to'
        + ' set it as the active config.');
    }

    if (!md.id) {
      throw new Error('The provided model must have an id in order to be set as the'
        + ' active config.');
    }

    if (this._active !== md.id) {
      this._activeId = md.id;
      localStorage.activeServerConfig = md.id;
      this.trigger('activeServerChange', md);
      this.bindActiveServerChangeHandler();
    }
  }

  onActiveServerChange(md) {
    this.trigger('activeServerChange', md);
  }

  bindActiveServerChangeHandler() {
    if (this.activeServer) {
      this.activeServer.off('change', this.onActiveServerChange)
        .on('change', this.onActiveServerChange);
    }
  }

  migrate() {
    let builtInCount = 0;

    this.forEach((serverConfig) => {
      // Migrate any old "built in" configurations containing the 'default' flag to
      // use the new 'builtIn' flag.
      const isDefault = serverConfig.get('default');

      if (typeof isDefault === 'boolean') {
        serverConfig.unset('default');
        const configSave = serverConfig.save({ builtIn: isDefault });

        if (!configSave) {
          // developer error or wonky data
          console.error('There was an error migrating the server config, '
            + `${serverConfig.get('name')}, from the 'default' to the 'built-in' style.`);
        }
      }

      if (serverConfig.get('builtIn')) {
        if (serverConfig.get('port') === 4002 && process.env.TESTNET !== 'true') {
          const configSave = serverConfig.save({ port: 5102 });

          if (!configSave) {
            // developer error or wonky data
            console.error('There was an error migrating the server config, '
              + `${serverConfig.get('name')}, from the port 4002 to 5102.`);
          }
        } else if (serverConfig.get('port') === 5102 && process.env.TESTNET === 'true') {
          const configSave = serverConfig.save({ port: 4002 });

          if (!configSave) {
            // developer error or wonky data
            console.error('There was an error migrating the server config, '
              + `${serverConfig.get('name')}, from the port 5102 to 4002.`);
          }
        }

        builtInCount += 1;
      }
    });

    // If there is just one built-in server, we'll ensure it has the correct name. If there
    // are multiple, which means they are legazy ones for different walletCurrencies, we'll
    // leave them be so the name still includes the currency and the user could still distinguish
    // between the two.
    if (builtInCount === 1) {
      const builtIn = this.findWhere({ builtIn: true });
      const configSave = builtIn.save({
        name: app.polyglot.t('connectionManagement.builtInServerName'),
      });

      if (!configSave) {
        // developer error or wonky data
        console.error('There was an error updating the name for built-in server '
          + `config ${builtIn.get('name')}.`);
      }
    }
  }
}
