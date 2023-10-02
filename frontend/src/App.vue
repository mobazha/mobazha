<template>
  <div class="app-container">
    <section id="pageNavContainer"></section>
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

    <ShoppingCart v-if="initialized && showShoppingCart" @close="onShoppingCartClose" @openPurchaseModal="onOpenPurchaseModal"/>
    <Wallet
      v-if="initialized"
      v-show="showWallet"
      :bb="function() {
        return {
          walletBalances: app.walletBalances,
        };
      }"
      @close="onWalletClose" />
    <Purchase v-if="showPurchase" @close="onPurchaseClose" />
    <LoadingModal v-if="initialized" v-show="showLoadingModal" />
  </div>
</template>

<script>
import ShoppingCart from '@/views/ShoppingCart.vue';
import Wallet from '@/views/modals/wallet/Wallet.vue';
import Purchase from './views/modals/purchase/Purchase.vue';
import LoadingModal from './views/modals/Loading.vue';
import app from '../backbone/app';

export default {
  components: {
    ShoppingCart,
    Wallet,
    Purchase,
    LoadingModal,
  },
  name: 
    'App',
  data() {
    return {
      modalName: '',
      initialized: false,
      showLoadingModal: false,

      showShoppingCart: false,
      showWallet: false,
      showPurchase: false,

      toggleVue: false,

      app: app,
    };
  },
  watch: {},
  methods: {
    onShoppingCartClose() {
      this.showShoppingCart = false;
    },
    onWalletClose() {
      this.showWallet = false;
    },
    onOpenPurchaseModal() {
      this.showShoppingCart = false;
      this.showPurchase = true;
    },
    onPurchaseClose() {
      this.showPurchase = false;
    }
  },
};
</script>
<style lang="less"></style>
