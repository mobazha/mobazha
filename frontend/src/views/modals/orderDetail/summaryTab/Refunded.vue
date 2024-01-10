<template>
  <div class="refunded rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.refund.heading') }}</h2>
    <template v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ ob.moment(ob.timestamp).format('lll') }}</span>
    </template>
    <div class="border clrBr padMd">
      <div class="flexVCent gutterH clrT">
        <div class="statusIconCol clrT">
          <template v-if="!ob.isCrypto">
            <span class="clrBr ion-ios-rewind"></span>
          </template>
          <CryptoIcon v-else :code="ob.paymentCoin" className="clrBr" />
        </div>
        <div class="flexExpand tx5">
          <div class="rowTn txB">{{ infoLine }}</div>
          <div class="flex gutterH">
            <div class="" style="flex-shrink: 0">{{ confirmationsText }}</div>
            <div class="" style="flex-shrink: 0;max-width: 80px">
              <div class="noOverflow">
                <template v-if="ob.blockChainTxUrl">
                  <a class="clrT2" :href="ob.blockChainTxUrl">{{ ob.transactionID }}</a>
                </template>
                <template v-else>
                  <div class="clrT2">{{ ob.transactionID }}</div>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import moment from 'moment';
import app from '../../../../../backbone/app';
import { abbrNum } from '../../../../../backbone/utils';


export default {
  props: {
    options: {
      type: Object,
      default: {
        buyerName: '',
        userCurrency: app.settings.get('localCurrency') || 'BTC',
        isCrypto: false,
        blockChainTxUrl: '',
      },
    },
    bb: Function,
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        buyerName: '',
        userCurrency: app.settings.get('localCurrency') || 'BTC',
        isCrypto: false,
        blockChainTxUrl: '',
        ...this.options,
        ...this.model,
        abbrNum,
        moment,
      };
    },
    infoLine () {
      const ob = this.ob;

      const divisibility = ob.currencyMod.getCoinDivisibility(ob.paymentCoin);
      const amount = ob.currencyMod.integerToDecimal(ob.amount, divisibility);
      const priceFrag = ob.currencyMod.pairedCurrency(
        amount,
        ob.paymentCoin,
        ob.userCurrency
      );

      if (ob.buyerName) {
        return ob.polyT(`orderDetail.summaryTab.payment.refundedTo`, {
          currencyPairing: priceFrag,
          payeeName: ob.buyerName,
        });
      } else {
        return ob.polyT(`orderDetail.summaryTab.payment.refunded`, {
          currencyPairing: priceFrag,
        });
      }
    },

    confirmationsText () {
      const ob = this.ob;

      if (ob.confirmations < 10000) {
        return ob.polyT('orderDetail.summaryTab.payment.confirmationsCount', {
          smart_count: ob.confirmations,
        });
      } else {
        return ob.polyT('orderDetail.summaryTab.payment.veryManyConfirmationsCount', {
          countText: ob.abbrNum(ob.confirmations),
        });
      }
    },
  },
  methods: {
    abbrNum, moment,
  }
}
</script>
<style lang="scss" scoped></style>
