<template>
  <div class="verifiedMod">
    <% const badgeFrag = `
    <div
      class="badge"
      style="background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');"
      tabindex="0"
    ></div>
    `; const warnFrag = `
    <div class="warning"><i class="ion-alert-circled clrTAlert"></i></div>
    `; const arrowBoxColor = ob.verified ? 'clrBrAlert2 clrBAlert2Grad' : 'clrP clrBr'; const arrowBoxClass = `${ob.arrowClass} ${arrowBoxColor}`; const
    arrowBox = `
    <div class="arrowBox ${arrowBoxClass}">
      <div class="titleWrapper ${ob.titleWrapperClass}">
        ${ob.verified ? badgeFrag : warnFrag}
        <div class="${ob.tipTitleClass}">${ob.tipTitle}</div>
      </div>
      <div class="${ob.tipBodyClass}">${ob.tipBody}</div>
    </div>
    `; let icon = `<i class="${ob.infoIconClass}"></i>`; if (ob.wrapInfoIcon) { icon = `
    <div class="arrowBoxTipWrap">${icon} ${arrowBox}</div>
    `; } const textWrapper = `
    <div class="textWrapper ${ob.textWrapperClass}">
      ${ob.verified ? badgeFrag : warnFrag} ${ ob.text ? `<span class="${ob.textClass}">${ob.text}${icon}</span>` : '' }
    </div>
    `; %>

    <div :class="`innerWrap ${ob.verified ? 'verified' : 'unverified'}`">
      <div :class="`arrowBoxTipWrap`" v-if="!ob.wrapInfoIcon">
        <div :class="`textWrapper ${ob.textWrapperClass}`">
          <div
            v-if="ob.verified"
            class="badge"
            :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`"
            tabindex="0"
          ></div>
          <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

          <span v-if="ob.text" :class="ob.textClass">${ob.text}<i :class="ob.infoIconClass"></i></span>
        </div>

        <div :class="`arrowBox ${arrowBoxClass}`">
          <div :class="`titleWrapper ${ob.titleWrapperClass}`">
            <div
              v-if="ob.verified"
              class="badge"
              :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`"
              tabindex="0"
            ></div>
            <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

            <div :class="ob.tipTitleClass">{{ ob.tipTitle }}</div>
          </div>
          <div :class="ob.tipBodyClass">{{ ob.tipBody }}</div>
        </div>
      </div>
      <div v-else>
        <div :class="`textWrapper ${ob.textWrapperClass}`">
          <div
            v-if="ob.verified"
            class="badge"
            :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`"
            tabindex="0"
          ></div>
          <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

          <span v-if="ob.text" :class="ob.textClass"
            >${ob.text}
            <div class="arrowBoxTipWrap">
              <i class="${ob.infoIconClass}"></i>

              <div class="arrowBox ${arrowBoxClass}">
                <div class="titleWrapper ${ob.titleWrapperClass}">
                  <div
                    v-if="ob.verified"
                    class="badge"
                    :style="`background-image: ${ob.badgeUrl ? `url(${ob.badgeUrl}),` : ''}url('../imgs/verifiedModeratorBadgeDefault-tiny.png');`"
                    tabindex="0"
                  ></div>
                  <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

                  <div :class="ob.tipTitleClass">{{ ob.tipTitle }}</div>
                </div>
                <div :class="ob.tipBodyClass">{{ ob.tipBody }}</div>
              </div>
            </div>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import app from '../../../backbone/app';
import VerifiedMod from '../../../backbone/models/VerifiedMod';
import loadTemplate from '../../../backbone/utils/loadTemplate';
import { isHiRez } from '../../../backbone/utils/responsive';
import { handleLinks } from '../../../backbone/utils/dom';
import BaseVw from '../baseVw';

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

    className() {
      return 'verifiedMod';
    },

    events() {
      return {
        click: 'onClick',
      };
    },

    onClick(e) {
      e.stopPropagation();
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
