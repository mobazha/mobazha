'use strict';

const { Controller } = require('ee-core');
const Log = require('ee-core/log');
const Services = require('ee-core/services');
import { clipboard } from 'electron';

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
  }
}

SystemController.toString = () => '[class SystemController]';
module.exports = SystemController;  