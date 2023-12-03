<template>
  <div class="modal settings tabbedModal modalScrollPage">
    <BaseModal :modalInfo="{ removeOnClose: true, removeOnRoute: false, }" @close="close">
      <template v-slot:component>
        <div class="topControls flex"></div>
        <div class="flex gutterH">
          <div class="tabColumn contentBox padMd clrP clrBr clrSh3">
            <h1 class="h4 txUp clrT">{{ ob.polyT('settings.settingsLabel') }}</h1>
            <div class="boxList tx4 clrTx1Br">
              <template v-for="tab in ['general', 'page', 'store', 'addresses', 'blocked', 'moderation', 'advanced']">
                <a :class="`tab row ${capitalize(tab) === activeTab ? 'clrT active' : ''}`" @click="tabClick(capitalize(tab))">{{ ob.polyT(`settings.${tab}Tab.sectionName`) }}</a>
              </template>
            </div>
          </div>
          <div class="flexExpand posR">
            <div class="js-settings-tabContent tabContent">
              <template v-for="tab in ['General', 'Page', 'Store', 'Addresses', 'Blocked', 'Moderation', 'Advanced']">
                <component :is="tab" :ref="tab" v-if="activeTab == tab" @unrecognizedModelError="onUnrecognizedModelError" />
              </template>
            </div>
          </div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import { recordEvent } from '../../../../backbone/utils/metrics';
import { capitalize } from '../../../../backbone/utils/string';

import General from './General.vue';
import Page from './Page.vue';
import Store from './Store.vue';
import Addresses from './Addresses.vue';
import Blocked from './Blocked.vue';
import Moderation from './Moderation.vue';
import Advanced from './advanced/Advanced.vue';


export default {
  components: {
    General,
    Page,
    Store,
    Addresses,
    Blocked,
    Moderation,
    Advanced,
  },
  props: {
    options: {
      type: Object,
      default: {
        scrollTo: '',
      },
    },
  },
  data () {
    return {
      initialTab: 'General',
      activeTab: '',

      currentTabView: undefined,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.$tabContent = $('.js-settings-tabContent');

    this.selectTab(this.initialTab, {
      scrollTo: this.options.scrollTo,
    });
  },
  computed: {
  },
  methods: {
    capitalize,
    loadData (options = {}) {
      this.baseInit(options);

      this.tabViewCache = {};
      this.tabViews = {
        General,
        Page,
        Store,
        Addresses,
        Advanced,
        Moderation,
        Blocked,
      };

      this.listenTo(app.router, 'will-route', () => {
        this.close(true);
        this.remove();
      });
    },

    tabClick (targ) {
      recordEvent('Settings_TabOpen', { tab: targ });
      this.selectTab(targ);
    },

    onUnrecognizedModelError (tabView, models = []) {
      const errors = models.map(md => {
        const errObj = md.validationError || {};
        return Object.keys(errObj).map(key => `${key}: ${errObj[key]}`);
      });

      const body = app.polyglot.t('settings.unrecognizedModelErrsWarning.body') +
        (errors.length ? `<br><br>${errors.join('<br> ')}` : '');

      openSimpleMessage(app.polyglot.t('settings.unrecognizedModelErrsWarning.title'), body);
    },

    selectTab (targ, options = {}) {
      const currentTab = this.activeTab;
      const targetTab = targ;

      if (!this.currentTabView || currentTab !== targetTab) {
        this.activeTab = targetTab;

        this.$nextTick(() => {
          this.currentTabView = this.$refs[targetTab];

          if (options.scrollTo && typeof this.currentTabView.scrollTo === 'function') {
            setTimeout(() => this.currentTabView.scrollTo(options.scrollTo));
          }
        }); 
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
