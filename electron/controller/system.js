'use strict';

const { Controller } = require('ee-core');
const Log = require('ee-core/log');
const Services = require('ee-core/services');
const { clipboard, shell } = require('electron');
const { platform, homedir } = require('os');

const fs = require('fs');
const path = require('path');

/**
 * example
 * @class
 */
class SystemController extends Controller {

  constructor(ctx) {
    super(ctx);
  }

  /**
   * 所有方法接收两个参数
   * @param args 前端传的参数
   * @param event - ipc通信时才有值
   * @param event - IpcMainEvent 文档：https://www.electronjs.org/docs/latest/api/structures/ipc-main-event
   */

  /**
   * test
   */
  async test (args, event) {

    // 前端参数
    const params = args;

    // 调用service
    const result = await Services.get('example').test('electron');
    Log.info('service result:', result);

    // 主动向前端发请求
    // channel 前端ipc.on()，监听的路由
    const channel = "controller.example.something"
    event.reply(channel, {age:21})

    // 返回数据
    return 'hello electron-egg';
  }

  /*
  */
  async writeToClipboard (text, event) {
    clipboard.writeText(text);
    Log.info('write content to clipboard');
  }

  async openExternal (href, event) {
    shell.openExternal(href);
    Log.info('open link externally, ', href);
  }

  getGlobal (key, event) {
    return global[key];
  }

  getPlatform (key, event) {
    return platform();
  }

  getHomedir (key, event) {
    return homedir();
  }

  async doMainWindowAction (action, event) {
    if (typeof this.app.electron.mainWindow[action] === 'function') {
      if (action == 'setFullScreen') {
        const isFullScreen = this.app.electron.mainWindow.isFullScreen();
        this.app.electron.mainWindow.setFullScreen(!isFullScreen);
      } else {
        this.app.electron.mainWindow[action]();
      }
    }
    Log.info('do action: ', action);
  }

  readTemplateFileSync (templateFile, event) {
    const root = '/Users/mingfeng/Downloads/tmp/electron-egg/frontend/backbone/js/templates';
    return fs.readFileSync(path.join(root, templateFile), 'utf8');
  }

  getlanguageFileContent (langFile, event) {
    const root = '/Users/mingfeng/Downloads/tmp/electron-egg/frontend/backbone/js/languages';
    return require(path.join(root, langFile));
  }

  async setProxy(args, event) {
    if (!args.id) {
      throw new Error('Please provide an id that will be passed back with the "proxy-set" '
        + 'event.');
    }

    this.app.electron.mainWindow.webContents.session.setProxy({
      proxyRules: args.socks5Setting,
      proxyBypassRules: '<local>',
    }).then(() => this.app.electron.mainWindow.webContents.send('proxy-set', args.id));
  }
}

SystemController.toString = () => '[class SystemController]';
module.exports = SystemController;  