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
              <Page ref="Page" v-if="activeTab == 'Page'" @unrecognizedModelError="onUnrecognizedModelError" />
              <Store ref="Store" v-if="activeTab == 'Store'" @unrecognizedModelError="onUnrecognizedModelError" />
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

import General from '../../../../backbone/views/modals/Settings/General';
import Advanced from '../../../../backbone/views/modals/Settings/advanced/Advanced'
import Addresses from '../../../../backbone/views/modals/Settings/Addresses'
import Moderation from '../../../../backbone/views/modals/Settings/Moderation';
import Blocked from '../../../../backbone/views/modals/Settings/Blocked';

// import General from './General.vue';
// import Page from './Page.vue';

import Page from './Page.vue';
import Store from './Store.vue';

// import Addresses from './Addresses.vue';
// import Advanced from './advanced/Advanced.vue';
// import Moderation from './Moderation.vue';
// import Blocked from './Blocked.vue';

export default {
  components: {
    Page,
    Store,
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
        if (this.currentTabView && currentTab !== 'Store' && currentTab !== 'Page') this.currentTabView.$el.detach();

        this.activeTab = targetTab;
        this.$nextTick(() => {
          let tabView;

          if (targetTab === 'Store') {
            tabView = this.$refs.Store;
          } else if (targetTab === 'Page') {
            tabView = this.$refs.Page;
          } else {
            tabView = this.tabViewCache[targetTab];
            if (!tabView) {
              tabView = this.createChild(this.tabViews[targetTab]);
              this.tabViewCache[targetTab] = tabView;
              tabView.render();
              this.listenTo(tabView, 'unrecognizedModelError', this.onUnrecognizedModelError);
            }

            this.$tabContent.append(tabView.$el);
          }

          this.currentTabView = tabView;

          if (options.scrollTo && typeof tabView.scrollTo === 'function') {
            setTimeout(() => tabView.scrollTo(options.scrollTo));
          }
        }); 
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
