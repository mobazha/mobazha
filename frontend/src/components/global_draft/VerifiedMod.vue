<template>
  <div class="verifiedMod" @click.stop>
    <div :class="`innerWrap ${ob.verified ? 'verified' : 'unverified'}`">
      <template v-if="!ob.wrapInfoIcon">
        <div class="arrowBoxTipWrap">
          {{ textWrapper }}
          {{ arrowBox }}
        </div>
      </template>
      <template v-else>
        {{ textWrapper }}
      </template>
    </div>
  </div>
</template>

<script>
import loadTemplate from '../../../backbone/utils/loadTemplate';
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
    return {};
  },
  created() {
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted() {
    this.render();
  },
  computed: {
    params() {
      return {
        ...this.getState(),
      };
    },
    badgeFrag() {
      let ob = this.ob;
      return `<div class="badge" style="background-image: ${
        ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''
      }url('../imgs/verifiedModeratorBadgeDefault-tiny.png');" tabindex="0"></div>`;
    },
    warnFrag() {
      return `<div class="warning"><i class="ion-alert-circled clrTAlert"></i></div>`;
    },
    arrowBoxClass() {
      let ob = this.ob;
      return `${ob.arrowClass} ${ob.verified ? 'clrBrAlert2 clrBAlert2Grad' : 'clrP clrBr'}`;
    },
    arrowBox() {
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
      let ob = this.ob;
      let icon = `<i class="${ob.infoIconClass}"></i>`;
      if (ob.wrapInfoIcon) return ` <div class="arrowBoxTipWrap">${icon} ${this.arrowBox}</div>`;
      return icon;
    },
    textWrapper() {
      let ob = this.ob;
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

      this.setState(opts.initialState || {});
      handleLinks(this.el);
    },
    render() {
      super.render();
      loadTemplate('/components/verifiedMod.html', (t) => {
        this.$el.html(
          t({
            ...this.getState(),
          })
        );
      });

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
