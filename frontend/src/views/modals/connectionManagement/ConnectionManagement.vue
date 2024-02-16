<template>
  <div class="modal connectionManagement tabbedModal modalScrollPage">
    <BaseModal>
      <template v-slot:component>
        <div class="topControls flex js-closeClickTarget" @click="close"></div>
        <div class="flex gutterH js-closeClickTarget" @click="close">
          <div class="tabColumn contentBox padMd clrP clrBr clrSh3">
            <h1 class="h4 txUp clrT">MENU</h1>
            <div class="boxList tx4 clrTx1Br">
              <a :class="`tab row ${activeTab === 'Configurations' ? 'clrT active' : ''}`" @click="onTabClick('Configurations')">{{ ob.polyT('connectionManagement.configurations.tabName') }}</a>
              <a :class="`tab row ${activeTab === 'ConfigForm' ? 'clrT active' : ''}`" @click="onTabClick('ConfigForm')">{{ ob.polyT('connectionManagement.configurationForm.tabName') }}</a>
            </div>

          </div>
          <div class="flexExpand">
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
import serverConnect from '../../../../backbone/utils/serverConnect';
import loadTemplate from '../../../../backbone/utils/loadTemplate';
import ServerConfig from '../../../../backbone/models/ServerConfig';
import BaseModal from '../BaseModal';
import Configurations from './Configurations';
import ConfigurationForm from './ConfigurationForm';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      activeTab: 'Configurations',
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        initialTabView: 'Configurations',
        ...options,
      };

      this.baseInit(opts);
      this.options = opts;

      this.tabViewCache = {};
      this.tabViews = {
        Configurations,
        ConfigurationForm,
      };
    },

    onTabClick (tab) {
      this.selectTab(tab);
    },

    createConfigurationsTabView () {
      const configTab = this.createChild(Configurations, {
        collection: app.serverConfigs,
      });

      this.listenTo(configTab, 'editConfig',
        e => this.selectTab('ConfigForm', { viewOptions: e || {} }));
      this.listenTo(configTab, 'newClick', () => this.selectTab('ConfigForm'));

      return configTab;
    },

    createConfigurationFormView (viewOptions = {}) {
      if (!viewOptions.model) {
        throw new Error('Please provide a server config model.');
      }

      const configForm = new ConfigurationForm({ ...viewOptions });
      this.listenTo(configForm, 'cancel', () => this.selectTab('Configurations'));
      this.listenTo(configForm, 'saved', () => {
        this.selectTab('Configurations');
        app.serverConfigs.add(configForm.model, { merge: true });
        serverConnect(configForm.model);
      });

      return configForm;
    },

    selectTab (tabViewName, data = {}) {
      let tabView = this.tabViewCache[tabViewName];
      data.viewOptions = data.viewOptions || {};

      if (!this.currentTabView || this.currentTabView !== tabView) {
        if (this.currentTabView) this.currentTabView.$el.detach();

        if (tabViewName === 'ConfigForm') {
          // we won't cache the Config Form tab and we'll manage it ourselves
          this.currentTabView =
            this.createConfigurationFormView({
              ...data.viewOptions,
              model: data.viewOptions.model || new ServerConfig(),
            });
          this.$tabContent.append(this.currentTabView.render().el);
        } else {
          if (!tabView) {
            if (this[`create${tabViewName}TabView`]) {
              tabView = this[`create${tabViewName}TabView`].apply(this, [data.viewOptions || {}]);
            } else {
              tabView = this.createChild(this.tabViews[tabViewName]);
            }

            this.tabViewCache[tabViewName] = tabView;
            tabView.render();
          }

          this.$tabContent.append(tabView.$el);
          this.currentTabView = tabView;
        }
      }
    },

    close () {
      this.selectTab('Configurations');
      
      this.$emit('close');
    },

    render () {
      loadTemplate('modals/connectionManagement/connectionManagement.html', t => {
        this.$el.html(t());
        super.render();

        this.$tabContent = $('.js-tabContent');

        this.selectTab(this.options.initialTabView);
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
