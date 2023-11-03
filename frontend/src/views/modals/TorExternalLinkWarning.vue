<template>
  <div class="modal modalScrollPage modalMedium">
    <BaseModal>
      <template v-slot:component>
        <div class="topControls flex"></div>

        <div class="contentBox padMd clrP clrBr clrSh3">
          <section>
            <h1 class="txCtr h3 pad">{{ ob.polyT('torExternalLinkWarning.heading') }}</h1>
            <hr class="clrBr rowHg" />
            <p>{{ ob.polyT('torExternalLinkWarning.body') }}</p>
            <div class="contentBox pad clrS clrBr row">{{ ob.url }}</div>
            <p class="rowHg">{{ ob.polyT('torExternalLinkWarning.areYouSureToProceed') }}</p>
            <hr class="clrBr row" />
            <div class="flexVCent">
              <div class="flexNoShrink tx5">
                <input type="checkbox" class="centerLabel" id="dontShowTorExternalLinkWarning">
                <label for="dontShowTorExternalLinkWarning">{{ ob.polyT('torExternalLinkWarning.doNotShowAgain') }}</label>
              </div>
              <div class="flexExpand flexHRight flexVCent gutterHMd">
                <a class="clrT2" @click="onCancelClick">{{ ob.polyT('torExternalLinkWarning.btnCancel') }}</a>
                <button class="btn clrP clrBr" @click="onConfirmClick">{{ ob.polyT('torExternalLinkWarning.btnProceed') }}</button>
              </div>
            </div>
          </section>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../backbone/app';

export default {
  props: {
    options: {
      type: Object,
      default: {
        url: '',
      },
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
    ob () {
      return {
        ...this.templateHelpers,
        url: this.url,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.url) {
        throw new Error('Please provide the url which the user is attempting to navigate to.');
      }

      this.baseInit(options);
    },

    onCancelClick () {
      this.$emit('cancelClick');
    },

    onConfirmClick () {
      this.$emit('confirmClick');
    },

    close () {
      if ($('#dontShowTorExternalLinkWarning').is(':checked')) {
        app.localSettings.save('dontShowTorExternalLinkWarning', true);
      }

      this.$emit('close');
    },
  }
}
</script>
<style lang="scss" scoped></style>
