<template>
  <tr :class="!ob.read ? 'unread' : ''" @click="onRowClick">
    <td class="clrBr orderCol noOverflow">
      <div class="unreadBorder clrE1"></div>
      <span class="ulOnHover">{{ ob.type === 'cases' ? ob.caseID : ob.orderID }}</span>
    </td>
    <td class="clrBr dateCol">
      <span class="ulOnHover">{{ ob.moment(ob.timestamp).format('l LT') }}</span>
    </td>

    <td v-if="ob.type !== 'cases'" class="clrBr listingCol js-listingCol" @click.stop="onClickListingColLink">
      <template v-if="!ob.coinType">
        <div class="flexVCent gutterHSm">
          <a :href="`#${ob.vendorID}/store/${ob.slug}`" class="thumb" :style="ob.getListingBgImage({ small: ob.thumbnail, tiny: ob.thumbnail })"></a>
          <a :href="`#${ob.vendorID}/store/${ob.slug}`" class="noOverflow clrT">{{ ob.title }}</a>
        </div>
      </template>
      <template v-else>
        <div class="flexVCent gutterHSm">
          <a :href="`#${ob.vendorID}/store/${ob.slug}`" class="clrT flexNoShrink js-cryptoTradingPairWrap">
            <CryptoTradingPairWrap :key="key" v-if="ob.coinType" :options="cryptoTradingPairOptions" />
          </a>
        </div>
      </template>
    </td>
    <td v-for="(user, index) in userCols" :key="index" class="clrBr userCol js-userCol" @click.stop="onClickUserColLink">
      <div class="flexVCent gutterHSm">
        <a class="avatar discSm clrBr2 clrSh1 flexNoShrink" :href="`#${user.userId}`" :style="ob.getAvatarBgImage(user.avatarHashes)"></a>
        <a class="handle noOverflow clrT" :href="`#${user.userId}`">{{ user.userHandle ? `@${user.userHandle}` : user.userId }}</a>
        <div class="flexHRight">
          <template v-if="ob.unreadChatMessages && index === 0">
            <span class="unreadBadge discSm clrE1 clrBrEmph1 clrTOnEmph">{{ ob.unreadChatMessages > 99 ? '…' : ob.unreadChatMessages }}</span>
          </template>
        </div>
      </div>
    </td>
    <td class="clrBr priceCol txRgt">
      <span class="ulOnHover">
        {{
          ob.currencyMod.convertAndFormatCurrency( ob.total, ob.paymentCoin, ob.userCurrency, { maxDisplayDecimals })
        }}
      </span>
    </td>
    <td class="clrBr gutterH statusCol">
      <template v-if="ob.state === 'PENDING'">
        <template v-if="ob.type === 'sales'">
          <span v-if="ob.rejectOrderInProgress" class="posR inlineBlock">
            <!-- // including invisible reject link to properly space the spinner -->
            <a class="txU tx6 invisible">{{ ob.polyT('transactions.transactionsTable.btnReject') }}</a>
            <SpinnerSVG className="spinnerSm center" />
          </span>
          <a v-else class="txU tx6" @click="onClickRejectOrder" :disabled="ob.acceptOrderInProgress">{{ ob.polyT('transactions.transactionsTable.btnReject') }}</a>

          <ProcessingButton
            :className="`js-acceptOrder btnAcceptOrder btn clrBAttGrad clrBrDec1 clrTOnEmph ${ob.acceptOrderInProgress ? 'processing' : ''}`"
            :disabled="ob.rejectOrderInProgress"
            @click="onClickAcceptOrder"
            :btnText= "ob.polyT('transactions.transactionsTable.btnAccept')"
          />
        </template>
        <template v-else-if="!ob.moderated">
          <!-- // Only non-moderated purchase can be canceled. We are not allowing PROCESSING_ERROR orders to be canceled here because
        // they need to be funded and we don't know if they are. If funded, they can be canceled on the Order Detail overlay. -->
          <span v-if="ob.cancelOrderInProgress" class="posR inlineBlock">
            <!-- // including invisible cancel link to properly space the spinner -->
            <a class="txU tx6 invisible">{{ ob.polyT('transactions.transactionsTable.btnCancel') }}</a>
            <SpinnerSVG className="spinnerSm center" />
          </span>
          <a v-else class="txU tx6 " @click.stop="onClickCancelOrder">{{ ob.polyT('transactions.transactionsTable.btnCancel')
          }}</a>
        </template>
        <template v-else>
          <span class="ulOnHover">{{ ob.polyT(`transactions.transactionsTable.status.${ob.state}`) }}</span>
        </template>
      </template>
      <template v-else>
        <span class="ulOnHover">{{ ob.polyT(`transactions.transactionsTable.status.${ob.state}`) }}</span>
      </template>
    </td>
  </tr>
</template>

<script>
/*
  This table is re-used for Sales, Purchases and Cases.
*/

import app from '../../../../backbone/app';
import moment from 'moment';
import _ from 'underscore';
import { recordEvent } from '../../../../backbone/utils/metrics';


export default {
  props: {
    options: {
      type: Object,
      default: {
        type: 'sales'
      },
    },
    bb: Function,
  },
  data () {
    return {
      key: 0,

      _state: {
        acceptOrderInProgress: false,
        rejectOrderInProgress: false,
        cancelOrderInProgress: false,
      }
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        type: this.type,
        ...this._state,
        ...this.model.toJSON(),
        userCurrency: app.settings.get('localCurrency'),
        moment,
        vendorID: this.type === 'sales' ? app.profile.id : this.model.get('vendorID'),
      }
    },
    userCols () {
      let ob = this.ob;

      const userCols = [];

      if (this.type !== 'sales') {
        userCols.push({
          avatarHashes: this.ob.vendorAvatarHashes || {},
          userHandle: this.ob.vendorHandle,
          userId: this.ob.vendorID,
        });
      }

      if (this.type !== 'purchases') {
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
        if (!this.ob.currencyMod.isFiatCur(this.ob.userCurrency)) {
          maxDisplayDecimals = 6;
        }
      } catch (e) {
        // pass
      }
      return maxDisplayDecimals;
    },
    cryptoTradingPairOptions() {
      const coinType = this.model.get('coinType');
      const paymentCoin = this.model.get('paymentCoin');
      let tradingPairClass = 'cryptoTradingPairSm';

      if (paymentCoin.length > 5 && coinType.length > 5) {
        tradingPairClass += ' longCurCodes';
      }

      return {
        tradingPairClass,
        exchangeRateClass: 'hide',
        fromCur: paymentCoin,
        toCur: coinType,
        truncateCurAfter: 5,
      };
    }
  },
  methods: {
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

      this.baseInit(opts);

      if (!this.model) {
        throw new Error('Please provide a model');
      }

      this.type = opts.type;

      this.listenTo(this.model, 'change', md => {
        if (md.hasChanged('read') &&
          Object.keys(md.changedAttributes).length === 1) {
          // if the only thing that has changed is the read flag,
          // we'll do nothing since that has it's own handler
          return;
        }

        this.key += 1;
      });
    },

    events () {
      return {
        'click .js-userCol a': 'onClickUserColLink',
        'click .js-listingCol a': 'onClickListingColLink',
      };
    },

    onClickAcceptOrder (e) {
      this.$emit('clickAcceptOrder', { view: this });
      recordEvent('Transactions_AcceptOrder');
    },

    onClickRejectOrder (e) {
      this.$emit('clickRejectOrder', { view: this });
      recordEvent('Transactions_RejectOrder');
    },

    onClickCancelOrder (e) {
      this.$emit('clickCancelOrder', { view: this });
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
      this.$emit('clickRow');
      recordEvent('Transactions_ClickOrder', {
        type: this.type,
      });
    },
  }
}
</script>
<style lang="scss" scoped></style>
