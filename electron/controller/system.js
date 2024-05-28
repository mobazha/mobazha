'use strict';

const { app } = require('electron');
const { Controller } = require('ee-core');
const Log = require('ee-core/log');
const Services = require('ee-core/services');
const Ps = require('ee-core/ps');

const { clipboard, shell } = require('electron');
const { platform } = require('os');

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

  readTemplateFileSync (templateFile, event) {
    let variablePath = 'frontend/backbone/templates'; // 打包前路径
    if (app.isPackaged) {
      variablePath = 'public/templates';
    }
    const rootDir = Ps.getHomeDir();
    return fs.readFileSync(path.join(rootDir, variablePath, templateFile), 'utf8');
  }

  getlanguageFileContent (langFile, event) {
    let variablePath = 'frontend/backbone/languages'; // 打包前路径
    if (app.isPackaged) {
      variablePath = 'public/languages';
    }
    const rootDir = Ps.getHomeDir();
    return require(path.join(rootDir, variablePath, langFile));
  }

  async printLog(args, event) {
    const { type, content } = args;

    switch (type) {
      case 'error':
        Log.error(content);
        break;
      case 'debug':
        Log.debug(content);
        break;
      case 'warn':
        Log.warn(content);
        break;
      default:
        Log.info(content);
        break;
    }
  }
}

SystemController.toString = () => '[class SystemController]';
module.exports = SystemController;  