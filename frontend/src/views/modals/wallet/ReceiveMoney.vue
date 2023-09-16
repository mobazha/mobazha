<template>
  <div class="receiveMoney padMd">
    <h2 class="h4 txUnl rowMd">{{ ob.polyT('wallet.receiveMoney.title') }}</h2>
    <div :class="`rowMd receiveQrCodeWrap ${ob.fetching ? 'invisible' : ''}`">
      <a @click="copyAddressToClipboard"><img :src="qrDataUri"></a>
    </div>
    <div :class="`rowMd ${ob.fetching ? 'invisible' : ''}`">
      <a class=" clrT clrBr tx5 addressText clamp" @click="copyAddressToClipboard">{{ ob.address }}</a>
      <span class="posR copyTextWrap">
        <a class="tx5b txU copyAddress " @click="copyAddressToClipboard">Copy</a>
        <span class="hide tx5b copiedText js-copiedText">Copied</span>
      </span>
    </div>
    <div class="spinnerWrap" v-show="!!ob.fetching">
      <SpinnerSVG />
    </div>
  </div>
</template>

<script>
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
      const coinType = this.getState().coinType;

      return {
        ...this.templateHelpers,
        ...this._state,
        qrDataUri,
        coinName: app.polyglot.t(`cryptoCurrencies.${coinType}`, { _: coinType }),
      };
    },
    qrDataUri () {
      // defaulting to an empty image - needed for proper spacing
      // when the spinner is showing
      let qrDataUri = 'data:image/gif;base64,R0lGODlhAQABAAAAACw=';
      const address = this.ob.address;
      const coinType = this.ob.coinType;
      let walletCur;

      try {
        walletCur = getWalletCurByCode(coinType);
      } catch (e) {
        // pass
      }

      if (address && walletCur) {
        qrDataUri = qr(walletCur.qrCodeText(address),
          { type: 7, size: 5, level: 'M' });
      }
      return qrDataUri;
    }
  },
  methods: {
    copyAddressToClipboard () {
      ipc.send('controller.system.writeToClipboard', this.getState().address);
      clearTimeout(this.copyTextTimeout);
      const $copyText = this.getCachedEl('.js-copyAddress')
        .addClass('invisible');
      const $copiedText = this.getCachedEl('.js-copiedText')
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
