<template>
  <div class="verifiedMod">
    <div :class="`innerWrap ${ob.verified ? 'verified' : 'unverified'}`">
      <template v-if="!ob.wrapInfoIcon">
        <div class="arrowBoxTipWrap">
          <div :class="`textWrapper ${ob.textWrapperClass}`">
            <img v-if="ob.verified" class="badge" :src="badgeUrlInfo" tabindex="0" />
            <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

            <span v-if="ob.text" class="${ob.textClass}">{{ob.text}}<i :class="ob.infoIconClass"></i></span>
          </div>

          <div :class="`arrowBox ${arrowBoxClass}`">
            <div :class="`titleWrapper ${ob.titleWrapperClass}`">
              <img v-if="ob.verified" class="badge" :src="badgeUrlInfo" tabindex="0" />
              <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

              <div :class="ob.tipTitleClass" v-html="ob.tipTitle"></div>
            </div>
            <div :class="ob.tipBodyClass" v-html="ob.tipBody"></div>
          </div>
        </div>
      </template>
      <template v-else>
        <div :class="`textWrapper ${ob.textWrapperClass}`">
          <img v-if="ob.verified" class="badge" :src="badgeUrlInfo" tabindex="0" />
          <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>

          <span v-if="ob.text" class="${ob.textClass}">
            {{ob.text}}
            <div class="arrowBoxTipWrap">
              <i :class="ob.infoIconClass"></i>

              <div :class="`arrowBox ${arrowBoxClass}`">
                <div :class="`titleWrapper ${ob.titleWrapperClass}`">
                  <img v-if="ob.verified" class="badge" :src="badgeUrlInfo" tabindex="0" />
                  <div v-else class="warning"><i class="ion-alert-circled clrTAlert"></i></div>
                  <div :class="ob.tipTitleClass" v-html="ob.tipTitle"></div>
                </div>
                <div :class="ob.tipBodyClass" v-html="ob.tipBody"></div>
              </div> 
            </div>
          </span>
        </div>
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

    arrowBoxClass() {
      const ob = this.ob;
      return `${ob.arrowClass} ${ob.verified ? 'clrBrAlert2 clrBAlert2Grad' : 'clrP clrBr'}`;
    },
    
    badgeUrlInfo() {
      const state = this._state;
      return state.badgeUrl ? state.badgeUrl : this.templateHelpers.getImagePath('verifiedModeratorBadgeDefault-tiny.png');
    }
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
