<template>
  <div class="app-container">
    <section id="pageNavContainer">
      <PageNav ref="pageNav" />
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

    <LoadingModal v-if="initialized" v-show="showLoadingModal" />
  </div>
</template>

<script>
import app from '../backbone/app';
import Settings from '@/views/modals/settings/Settings.vue';
import EditListing from '@/views/modals/editListing/EditListing.vue';
import ModeratorDetails from '@/views/modals/ModeratorDetails.vue'

import Wallet from '@/views/modals/wallet/Wallet.vue';
import ShoppingCart from '@/views/ShoppingCart.vue';
import Purchase from '@/views/modals/purchase/Purchase.vue';
import LoadingModal from '@/views/modals/Loading.vue';
import PageNav from '@/views/PageNav.vue'

import { init } from '@web3-onboard/vue'
import injectedModule from '@web3-onboard/injected-wallets'

export default {
  components: {
    Settings,
    EditListing,
    ModeratorDetails,

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

      toggleVue: false,

      app: app,

      modalName: '',
      modalOptions: {},
      modalBBFunc: undefined,
    };
  },
  created() {
    const injected = injectedModule();

    const infuraKey = '<INFURA_KEY>'
    const rpcUrl = `https://mainnet.infura.io/v3/${infuraKey}`

    init({
      wallets: [injected],
      chains: [
      // {
      //     id: '0x1',
      //     token: 'ETH',
      //     label: 'Ethereum Mainnet',
      //     rpcUrl
      //   },
      //   {
      //     id: 42161,
      //     token: 'ARB-ETH',
      //     label: 'Arbitrum One',
      //     rpcUrl: 'https://rpc.ankr.com/arbitrum'
      //   },
      //   {
      //     id: '0xa4ba',
      //     token: 'ARB',
      //     label: 'Arbitrum Nova',
      //     rpcUrl: 'https://nova.arbitrum.io/rpc'
      //   },
      //   {
      //     id: '0xa4ec',
      //     token: 'ETH',
      //     label: 'Celo',
      //     rpcUrl: 'https://1rpc.io/celo'
      //   },
      //   {
      //     id: 666666666,
      //     token: 'DEGEN',
      //     label: 'Degen',
      //     rpcUrl: 'https://rpc.degen.tips'
      //   },
        {
          id: 1030,
          token: 'CFX',
          label: 'Conflux',
          icon: 'imgs/cryptoIcons/CFX-icon.png',
          rpcUrl: 'https://evm.confluxrpc.com/GGna2h7aru3XSNFpeLrfT2ahYqM3YFeiX4FfgCgChdSfM9CbMXPik9762LBpKrzbC4c7kENDz2ikAYdyHQWjGiDvJ'
        },
        {
          id: 137,
          token: 'MATIC',
          label: 'Polygon',
          icon: 'imgs/cryptoIcons/MATIC-icon.png',
          rpcUrl: 'https://polygon-bor-rpc.publicnode.com'
        }
      ]
    })
  },
  mounted() {
  },
  watch: {},
  methods: {
  },
};
</script>
<style lang="less"></style>
