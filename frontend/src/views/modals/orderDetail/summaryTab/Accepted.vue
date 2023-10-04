<template>
  <div class="acceptedEvent rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.accepted.heading') }}</h2>
    <template v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ moment(ob.timestamp).format('lll') }}</span>
    </template>
    <div class="border clrBr padMd">
      <div class="flexVCent gutterH clrT">
        <div class="avatarCol disc clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.avatarHashes)"></div>
        <div class="flexExpand tx5">
          <div class="rowTn txB">{{ ob.polyT('orderDetail.summaryTab.accepted.orderAccepted') }}</div>
          <div>{{ infoText }}</div>
        </div>
        <div class="col">
          <div class="flexVCent gutterHLg">
            <template v-if="ob.showRefundButton">
              <template v-if="refundOrderInProgress">
                <span class="posR">
                  <!-- // including invisible refund link to properly space the spinner -->
                  <a class="txU tx6 invisible">{{ ob.polyT('orderDetail.summaryTab.accepted.refundBtn') }}</a>
                  <SpinnerSVG className="spinnerSm center" />
                </span>
              </template>

              <template v-else>
                <div class="posR">
                  <a class="txU tx6" :disabled="refundConfirmOn || fulfillInProgress" @click="onClickRefundOrder">{{
                    ob.polyT('orderDetail.summaryTab.accepted.refundBtn') }}</a>
                  <template v-if="refundConfirmOn">
                    <div class="confirmBox refundConfirm tx5 arrowBoxTop clrBr clrP clrT"
                      @click="onClickRefundConfirmBox">
                      <div class="tx3 txB rowSm">{{ ob.polyT('orderDetail.summaryTab.accepted.refundConfirm.title') }}
                      </div>
                      <p>
                        {{
                          ob.polyT('orderDetail.summaryTab.accepted.refundConfirm.body',
                            {
                              cur: ob.polyT(`cryptoCurrencies.${ob.paymentCoin}`, { _: ob.paymentCoin, }),
                            }
                          )
                        }}
                      </p>
                      <hr class="clrBr row" />
                      <div class="flexHRight flexVCent gutterHLg buttonBar">
                        <a @click="onClickRefundConfirmCancel">{{ ob.polyT('orderDetail.summaryTab.accepted.refundConfirm.btnCancel') }}</a>
                        <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph" @click="onClickRefundConfirmed">{{ ob.polyT('orderDetail.summaryTab.accepted.refundConfirm.btnConfirm') }}</a>
                      </div>
                    </div>
                  </template>
                </div>
              </template>
            </template>
            <template v-if="showFulfillButton">
              <ProcessingButton
                :className="`btn clrBAttGrad clrBrDec1 clrTOnEmph tx5b js-fulfillOrder ${fulfillInProgress ? 'processing' : ''}`"
                :disabled="refundOrderInProgress" :btnText="ob.polyT('orderDetail.summaryTab.accepted.fulfillBtn')"
                @click="onClickFulfillOrder" />
            </template>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import moment from 'moment';
import {
  fulfillingOrder,
  refundingOrder,
  refundOrder,
  events as orderEvents,
} from '../../../../../backbone/utils/order';
import { recordEvent } from '../../../../../backbone/utils/metrics';


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
      infoText: '',
      showRefundButton: false,
      showFulfillButton: false,
      avatarHashes: {},
      refundConfirmOn: false,
      paymentCoin: undefined,

      fulfillInProgress: false,
      refundOrderInProgress: false,
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
  },
  methods: {
    moment,

    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide the order id.');
      }

      this.orderID = options.orderID;

      this.fulfillInProgress = fulfillingOrder(this.orderID);
      this.refundOrderInProgress = refundingOrder(this.orderID);

      this.listenTo(orderEvents, 'fulfillingOrder', e => {
        if (e.id === this.orderID) {
          this.fulfillInProgress = true;
        }
      });

      this.listenTo(orderEvents, 'fulfillOrderComplete fulfillOrderFail', e => {
        if (e.id === this.orderID) {
          this.fulfillInProgress = false;
        }
      });

      this.listenTo(orderEvents, 'refundingOrder', e => {
        if (e.id === this.orderID) {
          this.refundOrderInProgress = true;
        }
      });

      this.listenTo(orderEvents, 'refundOrderComplete refundOrderFail', e => {
        if (e.id === this.orderID) {
          this.refundOrderInProgress = false;
        }
      });

      this.boundOnDocClick = this.onDocumentClick.bind(this);
      $(document).on('click', this.boundOnDocClick);
    },

    onClickRefundOrder () {
      recordEvent('OrderDetails_Refund');
      this.refundConfirmOn = true;
      return false;
    },

    onClickRefundConfirmBox () {
      // ensure event doesn't bubble so onDocumentClick doesn't
      // close the confirmBox.
      return false;
    },

    onClickRefundConfirmCancel () {
      recordEvent('OrderDetails_RefundCancel');
      this.refundConfirmOn = false;
    },

    onDocumentClick () {
      this.refundConfirmOn = false;
    },

    onClickRefundConfirmed () {
      recordEvent('OrderDetails_RefundConfirm');
      this.refundConfirmOn = false;
      refundOrder(this.orderID);
    },

    onClickFulfillOrder () {
      recordEvent('OrderDetails_Fulfill');
      this.$emit('clickFulfillOrder');
    },

    remove () {
      $(document).off('click', this.boundOnDocClick);
    },
  }
}
</script>
<style lang="scss" scoped></style>
