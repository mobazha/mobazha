<template>
  <div class="modal modalScrollPage modalMedium">
    <BaseModal :modalInfo="{
        removeOnClose: true,
        showCloseButton: false,
        dismissOnEscPress: false,
      }">
      <template v-slot:component>
        <div class="topControls flex"></div>

        <section class="contentBox padMd clrP clrBr clrSh3">
          <h1 class="h3 pad flexCent gutterHSm">
            <div v-html="ob.parseEmojis('😲')"></div>
            <div v-html="ob.parseEmojis('😱')"></div>
            <div>{{ ob.polyT('nsfwWarning.heading') }}</div>
          </h1>
          <hr class="clrBr rowHg" />
          <p>{{ ob.polyT('nsfwWarning.paragraph1') }}</p>
          <p class="rowHg">{{ ob.polyT('nsfwWarning.paragraph2') }}</p>
          <hr class="clrBr rowMd" />
          <div class="flexVCent gutterH">
            <div class="flexExpand gutterHSm">
              <input class="centerLabel js-checkboxNsfw" type="checkbox" id="NSFW_OFF" />
              <label for="NSFW_OFF" class="lineHeight1">{{ ob.polyT('nsfwWarning.nsfwOffLabel') }}</label>
            </div>
            <div class="flexNoShrink">
              <div class="flexHRight flexVCent gutterHMd">
                <a class="clrT2" @click="onCancelClick">{{ ob.polyT('nsfwWarning.btnCancel') }}</a>
                <button class="btn clrP clrBr clrSh1" @click="onProceedClick">{{ ob.polyT('nsfwWarning.btnProceed') }}</button>
              </div>
            </div>
          </div>
        </section>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../backbone/app';
import { myAjax } from '../../api/api';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);
      this.listenTo(app.settings, 'change:showNsfw', this.onChangeNsfw);
    },

    onCancelClick () {
      this.$emit('canceled');
      this.close();
    },

    onChangeNsfw (md, showNsfw) {
      if (showNsfw) this.close();
    },

    onProceedClick () {
      if ($('.js-checkboxNsfw').is(':checked')) {
        this.stopListening(app.settings, null, this.onChangeNsfw);
        app.settings.set('showNsfw', true);
        myAjax({
          type: 'PUT',
          url: app.getServerUrl('ob/preferences'),
          data: JSON.stringify({ showNsfw: true }),
          dataType: 'json',
        });
      }

      this.close();
    },
  }
}
</script>
<style lang="scss" scoped></style>
