<template>
  <div class="searchProviders flexRow gutterH">
    <div class="thumb discoverLogo flexNoShrink"></div>
    <div class="providersHeader flexNoShrink">
      <div class="flexVCent">
        <div>
          <div class="tx4 rowTn">Mobazha</div>
          <div class="tx6">{{ ob.polyT('search.title') }}</div>
        </div>
      </div>
    </div>
    <div class="providersBar flexExpand">
      <div :class="`providerWrapper gutterH ${!ob.showSelectDefault ? 'margR' :''}`">
        <template v-if="ob.showSelectDefault">
          <div class="selectingBox confirmBox arrowBoxTop clrP clrBr clrSh1">
            <h2>{{ ob.polyT('search.chooseDefaultTitle') }}</h2>
            <p class="tx5">{{ ob.polyT('search.chooseDefaultMsg') }}</p>
          </div>
        </template>
        <template v-for="(provider, j) in app.searchProviders" :key="j">
          <Provider :options="{
              model: provider,
              active: options.currentID === provider.id,
              showSelectDefault: options.showSelectDefault,
            }"
            @click="(md) => {
              $emit('activateProvider', md);
            }"/>
        </template>
      </div>
      <div class="posR flexVCent addWrapper js-addWrapper" v-show="ob.showAdd">
        <button class="thumb clrP clrBr clrSh2 addBtn" @click.stop="onClickOpenAdd"><i class="ion-ios-plus-empty"></i></button>
        <AddProvider :searchType="options.searchType" @newProviderSaved="onNewProviderSaved" v-show="showAddProviderModal" @close="onAddProviderModalClose"/>
      </div>
    </div>
    <div>
      <div class="flexVCent gutterHSm">
        <a class="btn barBtn flexNoShrink tx6 clrP clrBr clrSh2" href="#transactions/sales">{{
          ob.polyT('search.providers.transactions') }}</a>
        <a class="btn barBtn flexNoShrink tx6 clrP clrBr clrSh2" :href="`#${ob.peerID}`">{{
          ob.polyT('search.providers.myPage') }}</a>
      </div>
    </div>
  </div>
</template>

<script>

import app from '../../../backbone/app';
import { recordEvent } from '../../../backbone/utils/metrics';
import { searchTypes } from '../../../backbone/utils/search';
import Provider from './Provider.vue';
import AddProvider from './AddProvider.vue';

export default {
  components: {
    Provider,
    AddProvider,
  },
  props: {
    options: {
      type: Object,
      default: {
        searchType: '',
        currentID: '',
        showSelectDefault: false,
      },
    },
  },
  data () {
    return {
      app: app,
      showAddProviderModal: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        peerID: app.profile.get('peerID'),
        showAdd: app.searchProviders.length < app.searchProviders.maxProviders,
        ...this.options,
      }
    }
  },
  methods: {
    loadData(options = {}) {
      if (!searchTypes.includes(options.searchType)) {
        throw new Error('Please include a valid searchType.');
      }
      this.baseInit(options);
    },

    onClickOpenAdd() {
      this.showAddProviderModal = true;
      recordEvent('Discover_AddProvider');
    },

    onNewProviderSaved(md) {
      this.$emit('activateProvider', md);
    },

    onAddProviderModalClose() {
      this.showAddProviderModal = false;
    }
  }
}
</script>
<style lang="scss" scoped></style>
