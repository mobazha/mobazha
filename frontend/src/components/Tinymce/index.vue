<template>
  <Editor :id="tinymceId" :init="initOptions"></Editor>
</template>
<script>
import Editor from '@tinymce/tinymce-vue';
import 'tinymce/tinymce';

// Theme
import 'tinymce/themes/silver/theme'
import 'tinymce/icons/default'

// // Skins
// import 'tinymce/skins/ui/oxide/skin.min.css'
// import 'tinymce/skins/ui/oxide/content.min.css'
// import 'tinymce/skins/content/default/content.min.css'

// Plugins
import 'tinymce/plugins/advlist';
import 'tinymce/plugins/anchor';
import 'tinymce/plugins/autolink';
import 'tinymce/plugins/autosave';
import 'tinymce/plugins/colorpicker';
import 'tinymce/plugins/directionality';
import 'tinymce/plugins/emoticons';
import 'tinymce/plugins/emoticons/js/emojis';
import 'tinymce/plugins/fullscreen';
import 'tinymce/plugins/hr';
import 'tinymce/plugins/image';
import 'tinymce/plugins/imagetools';
import 'tinymce/plugins/link';
import 'tinymce/plugins/lists';
import 'tinymce/plugins/nonbreaking';
import 'tinymce/plugins/noneditable';
import 'tinymce/plugins/pagebreak';
import 'tinymce/plugins/paste';
import 'tinymce/plugins/preview';
import 'tinymce/plugins/print';
import 'tinymce/plugins/save';
import 'tinymce/plugins/searchreplace';
import 'tinymce/plugins/spellchecker';
import 'tinymce/plugins/tabfocus';
import 'tinymce/plugins/table';
import 'tinymce/plugins/textcolor';
import 'tinymce/plugins/textpattern';
import 'tinymce/plugins/visualblocks';
import 'tinymce/plugins/visualchars';
import 'tinymce/plugins/wordcount';

import plugins from './plugins';
import toolbar from './toolbar';

import app from '../../../backbone/app';
import { getTinymceLangFileNameByCode } from '../../../backbone/data/languages';

const lang = getTinymceLangFileNameByCode(app.localSettings.get('language'));

export default {
  name: 'Tinymce',
  components: {
    Editor,
  },
  props: {
    id: {
      type: String,
      default: function () {
        return 'vue-tinymce-' + +new Date() + ((Math.random() * 1000).toFixed(0) + '');
      },
    },
    toolbar: {
      type: Array,
      required: false,
      default() {
        return [];
      },
    },
    menubar: {
      type: String,
      default: 'file edit insert view format table',
    },
    height: {
      type: [Number, String],
      required: false,
      default: 500,
    },
    width: {
      type: [Number, String],
      required: false,
      default: 'auto',
    },
    placeholder: {
      type: String,
      required: false,
      default: '',
    },
  },
  data () {
    return {
      tinymceId: this.id,
    };
  },
  computed: {
    initOptions() {
      return {
        selector: `#${this.tinymceId}`,
        language: lang,
        language_url: lang ? `../tinymce/langs/${lang}.js` : null,

        skin: 'oxide',
        skin_url: '../tinymce/skins/ui/oxide',
        content_css: '../tinymce/skins/content/default/content.min.css',

        height: this.height,
        width: this.width,
        body_class: 'panel-body',
        object_resizing: false,
        toolbar: this.toolbar.length > 0 ? this.toolbar : toolbar,
        plugins,
        menubar: this.menubar,
        placeholder: this.placeholder,
        end_container_on_empty_block: true,
        powerpaste_word_import: 'clean',
        advlist_bullet_styles: 'square',
        advlist_number_styles: 'default',
        default_link_target: '_blank',
        link_title: false,
        // contextmenu: 'selectall copy paste cut undo redo bold align backcolor link image',
        content_style: 'body {font-size:14px;}',
        nonbreaking_force_tab: true, // inserting nonbreaking space &nbsp; need Nonbreaking Space Plugin
        fontsize_formats: '12px 14px 16px 18px 24px 36px 48px 56px 72px',

        // it will try to keep these URLs intact
        // https://www.tiny.cloud/docs-3x/reference/configuration/Configuration3x@convert_urls/
        // https://stackoverflow.com/questions/5196205/disable-tinymce-absolute-to-relative-url-conversions
        convert_urls: false,
      };
    }
  },
  created() {
  }
};
</script>
<style lang="scss" scoped></style>
  