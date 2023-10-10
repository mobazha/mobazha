<template>
  <div class="modal settings tabbedModal modalScrollPage">
    <BaseModal :modalInfo="{ removeOnClose: true, removeOnRoute: false, }">
      <template v-slot:component>
        <div class="topControls flex"></div>
        <div class="flex gutterH">
          <div class="tabColumn contentBox padMd clrP clrBr clrSh3">
            <h1 class="h4 txUp clrT">{{ ob.polyT('settings.settingsLabel') }}</h1>
            <div class="boxList tx4 clrTx1Br">
              <template v-for="tab in ['general', 'page', 'store', 'addresses', 'blocked', 'moderation', 'advanced']">
                <a :class="`tab clrT ${tab === activeTab ? 'row active' : ''}`" @click="tabClick(ob.capitalize(tab))">{{ ob.polyT(`settings.${tab}Tab.sectionName`) }}</a>
              </template>
            </div>
          </div>
          <div class="flexExpand posR">
            <div class="js-tabContent tabContent"></div>
          </div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import { openSimpleMessage } from '../SimpleMessage';
import { recordEvent } from '../../../../backbone/utils/metrics';
import BaseModal from '../BaseModal';
import General from './General';
import Page from './Page';
import Store from './Store';
import Addresses from './Addresses';
import Advanced from './advanced/Advanced';
import Moderation from './Moderation';
import Blocked from './Blocked';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      activeTab: 'General',
      scrollTo: '',
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.$tabContent = $('.js-tabContent');

    this.selectTab($(`.js-tab[data-tab="${this.activeTab}"]`), {
      scrollTo: this.options.scrollTo,
    });
  },
  computed: {
  },
  methods: {
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
      const tabViewName = targ;
      let tabView = this.tabViewCache[tabViewName];

      if (!this.currentTabView || this.currentTabView !== tabView) {
        if (this.currentTabView) this.currentTabView.$el.detach();

        if (!tabView) {
          tabView = this.createChild(this.tabViews[tabViewName]);
          this.tabViewCache[tabViewName] = tabView;
          tabView.render();
          this.listenTo(tabView, 'unrecognizedModelError', this.onUnrecognizedModelError);
        }

        this.$tabContent.append(tabView.$el);
        this.currentTabView = tabView;

        if (options.scrollTo && typeof tabView.scrollTo === 'function') {
          setTimeout(() => tabView.scrollTo(options.scrollTo));
        }
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
