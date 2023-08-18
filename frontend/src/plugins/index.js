import vue from '@vitejs/plugin-vue';
import AutoImport from 'unplugin-auto-import/vite';
import vueSetupExtend from 'vite-plugin-vue-setup-extend';

function createAutoImport() {
  return AutoImport({
    imports: ['vue', 'vue-router', 'pinia'],
    dts: false,
  });
}
export default [vue(), vueSetupExtend(), createAutoImport()];
