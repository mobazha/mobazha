<template>
  <tr :class="!model.get('read') ? 'unread' : ''" @click="onRowClick">
    <td class="clrBr orderCol noOverflow">
      <div class="unreadBorder clrE1"></div>
      <span class="ulOnHover">{{ type === 'cases' ? info.caseID : info.orderID }}</span>
    </td>
    <td class="clrBr dateCol">
      <span class="ulOnHover">{{ moment(info.timestamp).format('l LT') }}</span>
    </td>

    <td v-if="type !== 'cases'" class="clrBr listingCol js-listingCol" @click.stop @click="onClickListingColLink">
      <div v-if="!info.coinType">
        <div class="flexVCent gutterHSm">
          <a :href="`#${vendorID}/store/${info.slug}`" class="thumb"
            :style="ob.getListingBgImage({ small: info.thumbnail, tiny: info.thumbnail })"></a>
          <a :href="`#${vendorID}/store/${info.slug}`" class="noOverflow clrT">{{ info.title }}</a>
        </div>
      </div>
      <div v-else>
        <div class="flexVCent gutterHSm">
          <a :href="`#${vendorID}/store/${info.slug}`" class="clrT flexNoShrink js-cryptoTradingPairWrap"></a>
        </div>
      </div>
    </td>
    <td v-for="(user, index) in userCols" :key="index" class="clrBr userCol js-userCol" @click.stop @click="onClickUserColLink">
      <div class="flexVCent gutterHSm">
        <a class="avatar discSm clrBr2 clrSh1 flexNoShrink" :href="`#${user.userId}`" :style="ob.getAvatarBgImage(user.avatarHashes)"></a>
        <a class="handle noOverflow clrT" :href="`#${user.userId}`">{{ user.userHandle ? `@${user.userHandle}` : user.userId }}</a>
        <div class="flexHRight">
          <div v-if="info.unreadChatMessages && index === 0">
            <span class="unreadBadge discSm clrE1 clrBrEmph1 clrTOnEmph">{{ info.unreadChatMessages > 99 ? '…' : info.unreadChatMessages }}</span>
          </div>
        </div>
      </div>
    </td>
    <td class="clrBr priceCol txRgt">
      <span class="ulOnHover">
        {{
          ob.currencyMod.convertAndFormatCurrency( info.total, info.paymentCoin, userCurrency, { maxDisplayDecimals })
        }}
      </span>
    </td>
    <td class="clrBr gutterH statusCol">
      <div v-if="info.state === 'PENDING'">
        <div v-if="type === 'sales'">
          <span v-if="info.rejectOrderInProgress" class="posR inlineBlock">
            <!-- // including invisible reject link to properly space the spinner -->
            <a class="txU tx6 invisible">{{ ob.polyT('transactions.transactionsTable.btnReject') }}</a>
            <SpinnerSVG className="spinnerSm center" />
          </span>
          <a v-else class="txU tx6" @click="onClickRejectOrder" :disabled="info.acceptOrderInProgress">{{ ob.polyT('transactions.transactionsTable.btnReject') }}</a>

          <ProcessingButton
            :className="`js-acceptOrder btnAcceptOrder btn clrBAttGrad clrBrDec1 clrTOnEmph ${info.acceptOrderInProgress ? 'processing' : ''}`"
            :disabled="rejectOrderInProgress"
            @click="onClickAcceptOrder"
            :btnText= "ob.polyT('transactions.transactionsTable.btnAccept')"
          />
        </div>
        <div v-else-if="!info.moderated">
          <!-- // Only non-moderated purchase can be canceled. We are not allowing PROCESSING_ERROR orders to be canceled here because
        // they need to be funded and we don't know if they are. If funded, they can be canceled on the Order Detail overlay. -->
          <span v-if="info.cancelOrderInProgress" class="posR inlineBlock">
            <!-- // including invisible cancel link to properly space the spinner -->
            <a class="txU tx6 invisible">{{ ob.polyT('transactions.transactionsTable.btnCancel') }}</a>
            <SpinnerSVG className="spinnerSm center" />
          </span>
          <a v-else class="txU tx6 " @click="onClickCancelOrder">{{ ob.polyT('transactions.transactionsTable.btnCancel')
          }}</a>
        </div>
        <div v-else>
          <span class="ulOnHover">{{ ob.polyT(`transactions.transactionsTable.status.${info.state}`) }}</span>
        </div>
      </div>
      <div v-else>
        <span class="ulOnHover">{{ ob.polyT(`transactions.transactionsTable.status.${info.state}`) }}</span>
      </div>
    </td>
  </tr>
</template>

<script>
/*
  This table is re-used for Sales, Purchases and Cases.
*/

import app from '../../../../backbone/app';
import moment from 'moment';
import { recordEvent } from '../../../../backbone/utils/metrics';
import CryptoTradingPair from '../../../../backbone/views/components/CryptoTradingPair';


export default {
  mixins: [],
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
    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
    info () {
      if (this.options.model && typeof this.options.model.toJSON === 'function') {
        return this.options.model.toJSON();
      }
      return {}
    },
    userCurrency () {
      return app.settings.get('localCurrency');
    },
    vendorID () {
      return this.type === 'sales' ? app.profile.id : this.model.get('vendorID');
    },
    userCols () {
      const userCols = [];

      if (this.type !== 'sales') {
        userCols.push({
          avatarHashes: ob.vendorAvatarHashes || {},
          userHandle: ob.vendorHandle,
          userId: this.vendorID,
        });
      }

      if (type !== 'purchases') {
        userCols.push({
          avatarHashes: ob.buyerAvatarHashes || {},
          userHandle: ob.buyerHandle,
          userId: ob.buyerID,
        });
      }
      return userCols;
    },
    maxDisplayDecimals () {
      let maxDisplayDecimals;

      try {
        if (!ob.currencyMod.isFiatCur(this.userCurrency)) {
          maxDisplayDecimals = 6;
        }
      } catch (e) {
        // pass
      }
      return maxDisplayDecimals;
    },
  },
  methods: {
    moment,

    loadData (options = {}) {
      const types = ['sales', 'purchases', 'cases'];
      const opts = {
        initialState: {
          acceptOrderInProgress: false,
          rejectOrderInProgress: false,
          cancelOrderInProgress: false,
        },
        type: 'sales',
        ...options,
      };

      if (types.indexOf(opts.type) === -1) {
        throw new Error('Please provide a valid type.');
      }

      if (!this.model) {
        throw new Error('Please provide a model');
      }

      this.type = opts.type;
    },

    events () {
      return {
        'click .js-userCol a': 'onClickUserColLink',
        'click .js-listingCol a': 'onClickListingColLink',
      };
    },

    onClickAcceptOrder (e) {
      this.$emit('clickAcceptOrder', { view: this });
      e.stopPropagation();
      recordEvent('Transactions_AcceptOrder');
    },

    onClickRejectOrder (e) {
      this.$emit('clickRejectOrder', { view: this });
      e.stopPropagation();
      recordEvent('Transactions_RejectOrder');
    },

    onClickCancelOrder (e) {
      this.$emit('clickCancelOrder', { view: this });
      e.stopPropagation();
      recordEvent('Transactions_CancelOrder');
    },

    onClickUserColLink () {
      recordEvent('Transactions_ClickUser', {
        type: this.type,
      });
    },

    onClickListingColLink () {
      recordEvent('Transactions_ClickListing', {
        type: this.type,
      });
    },

    onRowClick () {
      this.$emit('clickRow', { view: this });
      recordEvent('Transactions_ClickOrder', {
        type: this.type,
      });
    },

    render () {
      const coinType = this.model.get('coinType');

      if (coinType) {
        const paymentCoin = this.model.get('paymentCoin');
        let tradingPairClass = 'cryptoTradingPairSm';

        if (paymentCoin.length > 5 && coinType.length > 5) {
          tradingPairClass += ' longCurCodes';
        }

        if (this.cryptoTradingPair) this.cryptoTradingPair.remove();
        this.cryptoTradingPair = this.createChild(CryptoTradingPair, {
          initialState: {
            tradingPairClass,
            exchangeRateClass: 'hide',
            fromCur: paymentCoin,
            toCur: coinType,
            truncateCurAfter: 5,
          },
        });
        this.getCachedEl('.js-cryptoTradingPairWrap')
          .html(this.cryptoTradingPair.render().el);
      }

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
