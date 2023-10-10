<template>
  <div class="receiveMoney padMd">
    <h2 class="h4 txUnl rowMd">{{ ob.polyT('wallet.receiveMoney.title') }}</h2>
    <div :class="`rowMd receiveQrCodeWrap ${fetching ? 'invisible' : ''}`">
      <a @click="copyAddressToClipboard"><img :src="qrDataUri"></a>
    </div>
    <div :class="`rowMd ${fetching ? 'invisible' : ''}`">
      <a class=" clrT clrBr tx5 addressText clamp" @click="copyAddressToClipboard">{{ address }}</a>
      <span class="posR copyTextWrap">
        <a class="tx5b txU copyAddress " @click="copyAddressToClipboard">Copy</a>
        <span class="hide tx5b copiedText js-copiedText">Copied</span>
      </span>
    </div>
    <div class="spinnerWrap" v-show="!!fetching">
      <SpinnerSVG />
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import qr from 'qr-encode';
import { ipc } from '../../../utils/ipcRenderer.js';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../backbone/data/walletCurrencies.js';
import app from '../../../../backbone/app.js';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    fetching: {
      type: Boolean,
      default: true,
    },
    address: {
      type: String,
      default: '',
    },
    coinType: {
      type: String,
      default: '',
    },
  },
  data () {
    return {
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
        qrDataUri: this.qrDataUri,
        coinName: app.polyglot.t(`cryptoCurrencies.${this.coinType}`, { _: this.coinType }),
      };
    },
    qrDataUri () {
      // defaulting to an empty image - needed for proper spacing
      // when the spinner is showing
      let qrDataUri = 'data:image/gif;base64,R0lGODlhAQABAAAAACw=';
      let walletCur;

      try {
        walletCur = getWalletCurByCode(this.coinType);
      } catch (e) {
        // pass
      }

      if (this.address && walletCur) {
        qrDataUri = qr(walletCur.qrCodeText(this.address),
          { type: 7, size: 5, level: 'M' });
      }
      return qrDataUri;
    }
  },
  methods: {
    copyAddressToClipboard () {
      ipc.send('controller.system.writeToClipboard', this.address);
      clearTimeout(this.copyTextTimeout);
      const $copyText = $('.js-copyAddress')
        .addClass('invisible');
      const $copiedText = $('.js-copiedText')
        .stop()
        .show();

      this.copyTextTimeout = setTimeout(() => {
        $copiedText.hide();
        $copyText.removeClass('invisible');
      }, 1000);
    },
  }
}
</script>
<style lang="scss" scoped></style>
