'use strict';

const path = require('path');

/**
 * 默认配置
 */
module.exports = (appInfo) => {

  const config = {};

  /**
   * 开发者工具
   */
  config.openDevTools = false;

  /**
   * 应用程序顶部菜单
   */
  config.openAppMenu = 'dev-show';

  /**
   * 主窗口
   */
  config.windowsOption = {
    title: 'Mobazha',
    width: 1200,
    height: 760,
    minWidth: 1170,
    minHeight: 700,
    center: true,
    frame: false,
    webPreferences: {
      webSecurity: false, // 跨域问题 -> 打开注释
      contextIsolation: false, // false -> 可在渲染进程中使用electron的api，true->需要bridge.js(contextBridge)
      nodeIntegration: true,
      //preload: path.join(appInfo.baseDir, 'preload', 'bridge.js'),
    },
    icon: path.join(appInfo.home, 'public', 'images', 'logo.png'),
  };

  /**
   * ee框架日志
   */  
  config.logger = {
    encoding: 'utf8',
    level: 'INFO',
    outputJSON: false,
    buffer: true,
    enablePerformanceTimer: false,
    rotator: 'day',
    appLogName: 'ee.log',
    coreLogName: 'ee-core.log',
    errorLogName: 'ee-error.log' 
  }

  /**
   * 远程模式-web地址
   */    
  config.remoteUrl = {
    enable: false,
    url: 'http://electron-egg.kaka996.com/'
  };

  /**
   * 内置socket服务
   */   
  config.socketServer = {
    enable: false,
    port: 7070,
    path: "/socket.io/",
    connectTimeout: 45000,
    pingTimeout: 30000,
    pingInterval: 25000,
    maxHttpBufferSize: 1e8,
    transports: ["polling", "websocket"],
    cors: {
      origin: true,
    },
    channel: 'c1'
  };

  /**
   * 内置http服务
   */     
  config.httpServer = {
    enable: false,
    https: {
      enable: false, 
      key: '/public/ssl/localhost+1.key',
      cert: '/public/ssl/localhost+1.pem'
    },
    host: '127.0.0.1',
    port: 7071,
    cors: {
      origin: "*"
    },
    body: {
      multipart: true,
      formidable: {
        keepExtensions: true
      }
    },
    filterRequest: {
      uris:  [
        'favicon.ico'
      ],
      returnData: ''
    }
  };

  /**
   * 主进程
   */     
  config.mainServer = {
    protocol: 'file://',
    indexPath: '/public/dist/index.html',
    host: 'localhost',
    port: 7072,
  }; 

  /**
   * 硬件加速
   */
  config.hardGpu = {
    enable: true
  };

  /**
   * 异常捕获
   */
  config.exception = {
    mainExit: false,
    childExit: true,
    rendererExit: true,
  };

  /**
   * jobs
   */
  config.jobs = {
    messageLog: true
  };  

  /**
   * 插件功能
   */
  config.addons = {
    window: {
      enable: true,
    },
    tray: {
      enable: true,
      title: 'Mobazha',
      icon: '/public/images/system-tray.png',
      macIcon: '/public/images/system-tray-mac.png',
    },
    security: {
      enable: true,
    },
    awaken: {
      enable: true,
      protocol: 'ob',
      args: []
    },
    autoUpdater: {
      enable: true,
      windows: true, 
      macOS: true, 
      linux: false,
      options: {
        provider: 'github',
        owner: 'mobazha',
        repo: 'mobazha-desktop'
      },
      force: true,
    },
    localServer: {
      enable: true,
      port: 18080,
    }
  };

  return {
    ...config
  };
}
