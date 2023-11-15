<template>
  <div class="app-container">
    <section id="pageNavContainer">
      <PageNav ref="pageNav" :options="{
        serverConfigs: app.serverConfigs,
      }" />
    </section>
    <section id="contentFrame" class="clrBr">
      <div id="pageContainer">
        <router-view v-if="toggleVue && initialized" :key="$route.params[$route.meta.watchParam]" />
      </div>
    </section>
    <section id="statusBar"></section>
    <div id="chatCloseBtn" class="chatCloseBtn js-chatClose ion-ios-close-empty iconBtn clrP clrBr4 clrT2"></div>
    <div id="chatContainer"></div>
    <div id="chatConvoContainer" class="clrP clrBr3"></div>
    <div id="js-vueModal"></div>

    <Purchase ref="purchaseModal" v-if="showPurchase" :options="purchaseOptions" :bb="purchaseBBFunc" @close="onPurchaseClose" />

    <!-- <KeepAlive v-if="initialized" :exclude="['EditListing', 'Settings', 'About', 'ShoppingCart', 'Purchase']"> -->
      <component v-if="modalName" :is="modalName" :name="modalName" ref="modalInstance"
        :options="modalOptions"
        :bb="modalBBFunc"
        @close="closeModal">
      </component>
    <!-- </KeepAlive> -->

    <LoadingModal v-if="initialized" v-show="showLoadingModal" />
  </div>
</template>

<script>
import app from '../backbone/app';
import Settings from '@/views/modals/settings/Settings.vue';
import EditListing from '@/views/modals/editListing/EditListing.vue';

import Wallet from '@/views/modals/wallet/Wallet.vue';
import ShoppingCart from '@/views/ShoppingCart.vue';
import Purchase from '@/views/modals/purchase/Purchase.vue';
import LoadingModal from '@/views/modals/Loading.vue';
import PageNav from '@/views/PageNav.vue'

export default {
  components: {
    Settings,
    EditListing,

    Wallet,
    ShoppingCart,
    Purchase,
    LoadingModal,
    PageNav,
  },
  name: 
    'App',
  data() {
    return {
      initialized: false,
      showLoadingModal: false,

      showPurchase: false,

      toggleVue: false,

      app: app,

      purchaseOptions: {},
      purchaseBBFunc: undefined,

      modalName: '',
      modalOptions: {},
      modalBBFunc: undefined,
    };
  },
  mounted() {
  },
  watch: {},
  methods: {
    onPurchaseClose() {
      this.showPurchase = false;
    },
    launchPurchaseModal(options, bbFunc = undefined) {
      this.closeModal();

      this.purchaseOptions = options;
      this.purchaseBBFunc = bbFunc;
      this.showPurchase = true;

      return this.$refs.purchaseModal;
    },

    launchModal(modalName, options, bbFunc = undefined) {
      if (modalName === 'ShoppingCart') {
        this.showPurchase = false;
      }

      this.modalName = modalName;
      this.modalOptions = options;
      this.modalBBFunc = bbFunc;

      return this.$refs.modalInstance;
    },
    closeModal() {
      this.modalName = '';
    }
  },
};
</script>
<style lang="less"></style>
