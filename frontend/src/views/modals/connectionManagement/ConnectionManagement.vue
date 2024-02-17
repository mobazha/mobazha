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
            <div class="js-tabContent tabContent">
              <Configurations
                v-show="activeTab === 'Configurations'"
                :bb="() => {
                  return { collection: app.serverConfigs, }
                }"
                @editConfig="onEditConfig"
                @newClick="onNewConfigClick"
              />
              <ConfigurationForm
                v-show="activeTab === 'ConfigForm'"
                :key="configFormModel"
                @cancel="onConfigurationFormCancel"
                @saved="onConfigurationFormSaved"
              />
            </div>
          </div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../../backbone/app';
import serverConnect from '../../../../backbone/utils/serverConnect';
import ServerConfig from '../../../../backbone/models/ServerConfig';
import Configurations from './Configurations.vue';
import ConfigurationForm from './ConfigurationForm.vue';


export default {
  components: {
    Configurations,
    ConfigurationForm,
  },
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

      configFormModel: {},
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
      this.baseInit(options);
    },

    onTabClick (tab) {
      this.selectTab(tab);
    },

    onEditConfig(e) {
      this.selectTab('ConfigForm', { viewOptions: e || {} });
    },

    onNewConfigClick() {
      this.selectTab('ConfigForm');
    },

    onConfigurationFormCancel() {
      this.selectTab('Configurations');
    },

    onConfigurationFormSaved() {
      this.selectTab('Configurations');
      app.serverConfigs.add(configForm.model, { merge: true });
      serverConnect(configForm.model);
    },

    selectTab (tabViewName, data = {}) {
      data.viewOptions = data.viewOptions || {};

      if (tabViewName != this.activeTab) {
        if (tabViewName === 'ConfigForm') {
          this.configFormModel = data.viewOptions.model || new ServerConfig();
        }

        this.activeTab = tabViewName;
      }
    },

    close () {
      this.selectTab('Configurations');
      
      this.$emit('close');
    },

    render () {
      this.selectTab(this.activeTab);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
