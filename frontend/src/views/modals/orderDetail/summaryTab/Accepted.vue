<template>
  <div class="acceptedEvent rowLg" @click="onDocumentClick">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.accepted.heading') }}</h2>
    <template v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ ob.moment(ob.timestamp).format('lll') }}</span>
    </template>
    <div class="border clrBr padMd">
      <div class="flexVCent gutterH clrT">
        <div class="avatarCol disc clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.avatarHashes)"></div>
        <div class="flexExpand tx5">
          <div class="rowTn txB">{{ ob.polyT('orderDetail.summaryTab.accepted.orderAccepted') }}</div>
          <div>{{ ob.infoText }}</div>
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
                  <a class="txU tx6" :disabled="refundConfirmOn || fulfillInProgress" @click.stop="onClickRefundOrder">{{
                    ob.polyT('orderDetail.summaryTab.accepted.refundBtn') }}</a>
                  <template v-if="refundConfirmOn">
                    <div class="confirmBox refundConfirm tx5 arrowBoxTop clrBr clrP clrT"
                      @click.stop.prevent>
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
            <template v-if="ob.showFulfillButton">
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
import moment from 'moment';
import {
  fulfillingOrder,
  refundingOrder,
  refundOrder,
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
      _state: {
        infoText: '',
        showRefundButton: false,
        showFulfillButton: false,
        avatarHashes: {},
        
        paymentCoin: undefined,
      },
      refundConfirmOn: false,
      fulfillInProgress: false,
      refundOrderInProgress: false,
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
        ...this._state,
        moment,
      };
    }
  },
  methods: {
    moment,

    loadData (options = {}) {
      this.baseInit({
        ...options,
        initialState: {
          infoText: '',
          showRefundButton: false,
          showFulfillButton: false,
          avatarHashes: {},
          paymentCoin: undefined,
          ...options.initialState,
        },
      });

      if (!options.orderID) {
        throw new Error('Please provide the order id.');
      }

      this.orderID = options.orderID;

      this.fulfillInProgress = fulfillingOrder(this.orderID),
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

      this.refundOrderInProgress = refundingOrder(this.orderID),
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
    },

    onClickRefundOrder () {
      recordEvent('OrderDetails_Refund');
      this.refundConfirmOn = true;
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
  }
}
</script>
<style lang="scss" scoped></style>
