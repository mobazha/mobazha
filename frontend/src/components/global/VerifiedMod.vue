<template>
  <div class="verifiedMod" @click.stop>
    <div :class="`innerWrap ${ob.verified ? 'verified' : 'unverified'}`">
      <template v-if="!ob.wrapInfoIcon">
        <div class="arrowBoxTipWrap">
          <div v-html="textWrapper" />
          <div v-html="arrowBox" />
        </div>
      </template>
      <template v-else>
        <div v-html="textWrapper" />
      </template>
    </div>
  </div>
</template>

<script>
import { isHiRez } from '../../../backbone/utils/responsive';
import { handleLinks } from '../../../backbone/utils/dom';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      _state: {
        verified: false,
        text: '',
        textClass: 'txB tx5b',
        textWrapperClass: 'flexVCent gutterHTn',
        infoIconClass: 'ion-information-circled clrT2',
        tipTitle: '',
        tipTitleClass: 'tx4 txB',
        titleWrapperClass: 'flexCent rowSm gutterHTn',
        tipBody: '',
        tipBodyClass: '',
        arrowClass: 'arrowBoxCenteredTop',
        badgeUrl: '',
        wrapInfoIcon: false,
      }
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
    badgeFrag() {
      const ob = this.ob;
      return `<div class="badge" style="background-image: ${
        ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''
      }url('../imgs/verifiedModeratorBadgeDefault-tiny.png');" tabindex="0"></div>`;
    },
    warnFrag() {
      return `<div class="warning"><i class="ion-alert-circled clrTAlert"></i></div>`;
    },
    arrowBoxClass() {
      const ob = this.ob;
      return `${ob.arrowClass} ${ob.verified ? 'clrBrAlert2 clrBAlert2Grad' : 'clrP clrBr'}`;
    },
    arrowBox() {
      const ob = this.ob;
      return `
      <div class="arrowBox ${this.arrowBoxClass}">
        <div class="titleWrapper ${ob.titleWrapperClass}">
          ${ob.verified ? this.badgeFrag : this.warnFrag}
          <div class="${ob.tipTitleClass}">${ob.tipTitle}</div>
        </div>
        <div class="${ob.tipBodyClass}">${ob.tipBody}</div>
      </div>     
    `;
    },
    icon() {
      const ob = this.ob;
      let icon = `<i class="${ob.infoIconClass}"></i>`;
      if (ob.wrapInfoIcon) return ` <div class="arrowBoxTipWrap">${icon} ${this.arrowBox}</div>`;
      return icon;
    },
    textWrapper() {
      const ob = this.ob;
      return `
      <div class="textWrapper ${ob.textWrapperClass}">
        ${ob.verified ? this.badgeFrag : this.warnFrag}
        ${ob.text ? `<span class="${ob.textClass}">${ob.text}<i class="${ob.infoIconClass}"></i></span>` : ''}
      </div>
    `;
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        ...options,
        initialState: {
          verified: false,
          text: '',
          textClass: 'txB tx5b',
          textWrapperClass: 'flexVCent gutterHTn',
          infoIconClass: 'ion-information-circled clrT2',
          tipTitle: (options.initialState && typeof options.initialState.tipTitle === 'undefined' && options.initialState.text) || '',
          tipTitleClass: 'tx4 txB',
          titleWrapperClass: 'flexCent rowSm gutterHTn',
          tipBody: '',
          tipBodyClass: '',
          arrowClass: 'arrowBoxCenteredTop',
          badgeUrl: '',
          wrapInfoIcon: (options.initialState && options.initialState.text) || false,
          ...(options.initialState || {}),
        },
      };

      if (!opts.initialState.badgeUrl && typeof opts.badge === 'object') {
        opts.initialState.badgeUrl = isHiRez ? opts.badge.small : opts.badge.tiny;
      }

      this.baseInit(opts);
      handleLinks(this.$el);
    },
  },
};
</script>
<style lang="scss" scoped></style>
