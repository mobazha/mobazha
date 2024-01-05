<template>
  <div class="disputePayoutEvent rowLg" @click="onDocumentClick">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.disputePayout.heading') }}</h2>
    <template v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ ob.moment(ob.timestamp).format('lll') }}</span>
    </template>
    <div class="border clrBr padMd">
      <div class="flexRow row">
        <div class="col8 gutterV">
          <div class="flex gutterH clrT">
            <div class="avatarCol disc clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.buyerAvatarHashes)">
            </div>
            <div class="flexExpand tx5">
              <div class="rowTn txB">{{ partyInfo.partyHeadings.buyer }}</div>
              <div>{{ partyInfo.priceLines.buyer }}</div>
            </div>
          </div>
          <div class="flex gutterH clrT">
            <div class="avatarCol disc clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.vendorAvatarHashes)">
            </div>
            <div class="flexExpand tx5">
              <div class="rowTn txB">{{ partyInfo.partyHeadings.vendor }}</div>
              <div>{{ partyInfo.priceLines.vendor }}</div>
            </div>
          </div>
          <div class="flex gutterH clrT">
            <div class="avatarCol disc clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.moderatorAvatarHashes)">
            </div>
            <div class="flexExpand tx5">
              <div class="rowTn txB">{{ partyInfo.partyHeadings.moderator }}</div>
              <div>{{ partyInfo.priceLines.moderator }}</div>
            </div>
          </div>
        </div>
        <div class="col4 flexHRight">
          <div class="posR">
            <template v-if="ob.showAcceptButton">
              <ProcessingButton
                :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph tx5b js-acceptPayout ${acceptInProgress ? 'processing' : ''}`"
                :disabled="acceptConfirmOn" :btnText="ob.polyT('orderDetail.summaryTab.disputePayout.btnAcceptPayout')"
                @click.stop="onClickAcceptPayout" />
            </template>
            <template v-if="acceptConfirmOn">
              <div class="confirmBox acceptPayoutConfirm tx5 arrowBoxTop clrBr clrP clrT" @click.stop.prevent>
                <div class="tx3 txB rowSm">{{ ob.polyT('orderDetail.summaryTab.disputePayout.acceptPayoutConfirm.title') }}</div>
                <p>{{ ob.polyT('orderDetail.summaryTab.disputePayout.acceptPayoutConfirm.body') }}</p>
                <hr class="clrBr row" />
                <div class="flexHRight flexVCent gutterHLg buttonBar">
                  <a @click="onClickAcceptPayoutConfirmCancel">{{ ob.polyT('orderDetail.summaryTab.disputePayout.acceptPayoutConfirm.btnCancel') }}</a>
                  <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click="onClickAcceptPayoutConfirmed">{{ ob.polyT('orderDetail.summaryTab.disputePayout.acceptPayoutConfirm.btnConfirm') }}</a>
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>
      <div class="flex gutterH">
        <!-- avatar col is just a spacer here -->
        <div class="avatarCol disc invisible flexNoShrink"></div>
        <div class="flexExpand tx5">
          <div class="rowTn">{{ noteFromHeading }}</div>
          <div>{{ ob.verdict }}</div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import app from '../../../../../backbone/app';
import moment from 'moment';
import {
  acceptingPayout,
  acceptPayout,
  events as orderEvents,
} from '../../../../../backbone/utils/order';
import { recordEvent } from '../../../../../backbone/utils/metrics';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      userCurrency: app.settings.get('localCurrency') || 'USD',

      acceptConfirmOn: false,
      acceptInProgress: false,
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
        showAcceptButton: false,
        ...this.options,
        moment,
      };
    },

    partyInfo() {
      let partyInfo = {
        partyHeadings: {},
        priceLines: {},
      };

      const ob = this.ob;

      ['buyer', 'vendor', 'moderator'].forEach((type, index) => {
        partyInfo.partyHeadings[type] = ob[`${type}Name`] ?
          ob.polyT(`orderDetail.summaryTab.disputePayout.${type}HeadingWithName`, { name: ob[`${type}Name`] }) :
          ob.polyT(`orderDetail.summaryTab.disputePayout.${type}Heading`);

        if (!ob.releaseInfo) {
          return;
        }

        partyInfo.priceLines[type] = ob.currencyMod.pairedCurrency(
          ob.releaseInfo[`${type}Amount`],
          ob.paymentCoin,
          this.userCurrency
        );
      });

      return partyInfo;
    },
    
    noteFromHeading() {
      const ob = this.ob;

      return ob.moderatorName ?
        ob.polyT('orderDetail.summaryTab.disputePayout.noteFromHeadingWithName', { name: ob.moderatorName }) :
        ob.polyT('orderDetail.summaryTab.disputePayout.noteFromHeading');
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide the orderID');
      }
      this.orderID = options.orderID;

      this.acceptInProgress = acceptingPayout(this.orderID);
      this.listenTo(orderEvents, 'acceptingPayout', e => {
        if (e.id === this.orderID) {
          this.acceptInProgress = true;
        }
      });

      this.listenTo(orderEvents, 'acceptPayoutComplete acceptPayoutFail', e => {
        if (e.id === this.orderID) {
          this.acceptInProgress = false;
        }
      });
    },

    onDocumentClick () {
      this.acceptConfirmOn = false;
    },

    onClickAcceptPayout () {
      recordEvent('OrderDetails_DisputeAcceptClick');
      this.acceptConfirmOn = true;
    },

    onClickAcceptPayoutConfirmCancel () {
      recordEvent('OrderDetails_DisputeAcceptCancel');
      this.acceptConfirmOn = false;
    },

    onClickAcceptPayoutConfirmed () {
      recordEvent('OrderDetails_DisputeAcceptConfirm');
      this.acceptConfirmOn = false;
      acceptPayout(this.orderID);
    },

  }
}
</script>
<style lang="scss" scoped></style>
