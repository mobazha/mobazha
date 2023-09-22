<template>
  <div>
    <section id="pageNavContainer"></section>
    <section id="contentFrame" class="clrBr">
      <div id="pageContainer">
        <router-view v-if="toggleVue"/>
      </div>
    </section>
    <section id="statusBar"></section>
    <div id="chatCloseBtn" class="chatCloseBtn js-chatClose ion-ios-close-empty iconBtn clrP clrBr4 clrT2"></div>
    <div id="chatContainer"></div>
    <div id="chatConvoContainer" class="clrP clrBr3"></div>
    <div id="js-vueModal"></div>

    <ShoppingCart :ref="name" v-if="modalName === 'ShoppingCart'" />
    <Wallet v-if="initialized" v-show="showWallet" @close="onWalletClose" />
  </div>
</template>

<script>
import ShoppingCart from '@/views/ShoppingCart.vue';
import Wallet from '@/views/modals/wallet/Wallet.vue';

export default {
  components: {
    ShoppingCart,
    Wallet,
  },
  name: 'App',
  data() {
    return {
      modalName: '',
      initialized: false,

      showWallet: false,

      toggleVue: false,
    };
  },
  watch: {},
  methods: {
    onWalletClose() {
      this.showWallet = false;
    }
  }
}
</script>
<style lang="less"></style>