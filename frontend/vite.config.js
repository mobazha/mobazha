import { defineConfig } from 'vite';
import plugins from './src/plugins';
import path from 'path';
import fs from 'fs';
// https://vitejs.dev/config/
export default defineConfig(({ command, mode }) => {
  return {
    // 项目插件
    plugins,
    server: {
      host: true,
      port: 8088,
      proxy: {
        '/v1': {
          target: 'http://127.0.0.1:5102',
          changeOrigin: true,
          secure: false,
          // rewrite: (path) => path.replace(/^\/api/, '')
        },
        '/info': {
          target: 'https://mobazha.info',
          changeOrigin: true,
          secure: false,
          rewrite: (path) => path.replace(/^\/info/, '')
        },
      },
    },
    // 基础配置
    base: './',
    publicDir: 'public',
    resolve: {
      alias: {
        '@': path.resolve(__dirname, 'src'),
      },
    },
    css: {
      preprocessorOptions: {
        less: {
          modifyVars: {
            '@border-color-base': '#dce3e8',
          },
          javascriptEnabled: true,
        },
      },
    },
    build: {
      outDir: 'dist',
      assetsDir: 'assets',
      assetsInlineLimit: 4096,
      cssCodeSplit: true,
      brotliSize: false,
      sourcemap: false,
      minify: 'terser',
      terserOptions: {
        compress: {
          // 生产环境去除console及debug
          drop_console: false,
          drop_debugger: true,
        },
      },
    },
    define: {
      global: {}
    },
    plugins: [
      ...plugins,
      {
        name: 'handle-backbone-templates',
        enforce: 'pre',
        configureServer(server) {
          // 添加自定义中间件来处理模板文件
          server.middlewares.use((req, res, next) => {
            if (req.url?.includes('/backbone/templates/')) {
              const filePath = path.join(__dirname, req.url);
              try {
                const content = fs.readFileSync(filePath, 'utf-8');
                res.setHeader('Content-Type', 'text/plain');
                res.end(content);
              } catch (err) {
                next(err);
              }
              return;
            }
            next();
          });
        },
      }
    ]
  };
});
