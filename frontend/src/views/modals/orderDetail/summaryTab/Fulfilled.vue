<template>
  <div class="fulfilledEvent rowLg">
    <h2 class="tx4 margRTn">{{ ob.polyT('orderDetail.summaryTab.fulfilled.heading') }}</h2>
    <template v-if="ob.timestamp">
      <span class="clrT2 tx5b">{{ ob.moment(ob.timestamp).format('lll') }}</span>
    </template>
    <div class="border clrBr padMd">
      <template v-if="ob.contractType === 'PHYSICAL_GOOD'">
        <div class="flex gutterH clrT">
          <div class="statusIconCol"><span class="clrBr ion-cube"></span></div>
          <div class="flexExpand tx5">
            <template v-if="!ob.isLocalPickup">
              <div class="rowTn txB">{{ ob.polyT('orderDetail.summaryTab.fulfilled.shippedByLabel') }} <span>{{ physicalDelivery.shipper }}</span></div>
              <div class="row">
                <span>{{ ob.polyT('orderDetail.summaryTab.fulfilled.trackingNumberLabel') }}</span> {{ physicalDelivery.trackingNumber || ob.polyT('orderDetail.summaryTab.notApplicable') }}
                <template v-if="physicalDelivery.trackingNumber">
                  <a class="clrTEm" @click="onClickCopyText(physicalDelivery.trackingNumber, $event)" data-status-indicator=".js-trackingCopiedToClipboard">{{
                      ob.polyT('orderDetail.summaryTab.fulfilled.copyLink') }}</a>
                  <a class="hide js-trackingCopiedToClipboard">{{ ob.polyT('copiedToClipboard') }}</a>
                </template>
              </div>
            </template>
            <div class="rowTn txB">{{ ob.noteFromLabel }}</div>
            <div v-html="`${ ob.note ? ob.parseEmojis(ob.note) : ob.polyT('orderDetail.summaryTab.notApplicable') }`"></div>
          </div>
        </div>
      </template>

      <template v-else-if="ob.contractType === 'DIGITAL_GOOD'">
        <div class="flex gutterH clrT">
          <div class="statusIconCol clrT"><span class="clrBr ion-ios-folder"></span></div>
          <div class="flexExpand tx5">
            <div class="rowTn txB">{{ ob.polyT('orderDetail.summaryTab.fulfilled.digitalReadyForDlHeading') }}</div>
            <div class="row">
              {{ ob.polyT('orderDetail.summaryTab.fulfilled.digitalReadyForDlText') }}
            </div>
            <div class="rowTn txB">{{ ob.polyT('orderDetail.summaryTab.fulfilled.urlLabel') }}</div>
            <div :class="`${ob.showPassword ? 'row' : ''}`">
              <a class="clrTEm" :href="digitalDelivery.url" data-open-external>{{ digitalDelivery.url }}</a>
            </div>
            <template v-if="ob.showPassword">
              <div class="rowTn txB">{{ ob.polyT('orderDetail.summaryTab.fulfilled.passwordLabel') }}</div>
              <div class="row">{{ digitalDelivery.password || ob.polyT('orderDetail.summaryTab.notApplicable') }}</div>
            </template>
            <div class="rowTn txB">{{ ob.noteFromLabel }}</div>
            <div v-html="`${ ob.note ? ob.parseEmojis(ob.note) : ob.polyT('orderDetail.summaryTab.notApplicable') }`"></div>
          </div>
        </div>
      </template>

      <template v-else-if="ob.contractType === 'SERVICE'">
        <div class="flex gutterH clrT">
          <div class="statusIconCol clrT"><span class="clrBr ion-ios-body"></span></div>
          <div class="flexExpand tx5">
            <div class="rowTn txB">{{ ob.noteFromLabel }}</div>
            <div v-html="`${ ob.note ? ob.parseEmojis(ob.note) : ob.polyT('orderDetail.summaryTab.notApplicable') }`"></div>
          </div>
        </div>
      </template>

      <template v-else-if="ob.contractType === 'CRYPTOCURRENCY'">
        <div class="flex gutterH clrT">
          <div class="statusIconCol">
            <CryptoIcon :code="ob.coinType" className="clrBr"/>
          </div>
          <div class="flexExpand tx5 posR">
            <div class="rowTn txB">{{ ob.polyT('orderDetail.summaryTab.fulfilled.cryptoSentLabel', {
              coinTypeVerbose:
                coinTypeVerbose,
            }) }}</div>

            <div class="row">
              <span>{{ ob.polyT('orderDetail.summaryTab.fulfilled.transactionIDLabel') }}</span>
              <span class="clamp3 inline">{{ ob.encodedTxId }}</span>
              <a class="clrTEm  flexNoShrink" @click="onClickCopyText(ob.transactionID, $event)" data-status-indicator=".js-transactionIDCopiedToClipboard">{{
                  ob.polyT('orderDetail.summaryTab.fulfilled.copyLink') }}</a>
              <a class="hide js-transactionIDCopiedToClipboard">{{ ob.polyT('copiedToClipboard') }}</a>
            </div>
            <div class="rowTn txB">{{ ob.noteFromLabel }}</div>
            <div v-html="`${ ob.note ? ob.parseEmojis(ob.note) : ob.polyT('orderDetail.summaryTab.notApplicable') }`"></div>
          </div>
        </div>
      </template>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import moment from 'moment';
import { ipc } from '../../../../utils/ipcRenderer.js';
import 'velocity-animate';
import app from '../../../../../backbone/app.js';


export default {
  props: {
    options: {
      type: Object,
      default: {
        contractType: 'PHYSICAL_GOOD',
        isLocalPickup: false,
        showPassword: false,
        noteFromLabel: app.polyglot.t('orderDetail.summaryTab.fulfilled.noteFromVendorLabel'),
        coinType: '',
      },
    },
    bb: Function
  },
  data () {
    return {
      dataObject: {
        cryptocurrencyDelivery: undefined,
        physicalDelivery: undefined,
        digitalDelivery: undefined,
      },
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
      const cd = this.dataObject.cryptocurrencyDelivery;
      const transactionID = cd && cd[0] && cd[0].transactionID || '';

      return {
        ...this.templateHelpers,
        contractType: 'PHYSICAL_GOOD',
        isLocalPickup: false,
        showPassword: false,
        noteFromLabel: app.polyglot.t('orderDetail.summaryTab.fulfilled.noteFromVendorLabel'),
        coinType: '',
        ...this.options,
        ...this.options.dataObject,
        ...this.dataObject,
        transactionID: transactionID.replace(/["]/g, '[!$quote$!]'),
        encodedTxId: this.revealEscapeChars(transactionID),
        moment,
      };
    },
    physicalDelivery () {
      return this.dataObject.physicalDelivery || {};
    },
    digitalDelivery () {
      return this.dataObject.digitalDelivery || {};
    },
    coinTypeVerbose () {
      const ob = this.ob;
      let coinTypeTranslationKey = `cryptoCurrencies.${ob.coinType}`;
      return ob.polyT(coinTypeTranslationKey) === coinTypeTranslationKey ?
        ob.coinType :
        ob.polyT('orderDetail.summaryTab.fulfilled.coinTypeVerbose', {
          coinName: ob.polyT(coinTypeTranslationKey),
          coinCode: ob.coinType,
        })
    },
    transactionID () {
      const cd = this.dataObject.cryptocurrencyDelivery;
      return cd && cd[0] && cd[0].transactionID || '';
    },
  },
  methods: {
    moment,

    loadData (options = {}) {
      if (!options.dataObject) {
        throw new Error('Please provide a vendorOrderFulfillment data object.');
      }

      this.dataObject = options.dataObject.fulfillments[0];
    },

    onClickCopyText (content, event) {
      const target = event.target;
      ipc.send('controller.system.writeToClipboard', content.replace(/\[!\$quote\$!\]/g, '"'));
      $(target.attr('data-status-indicator'))
        .velocity('stop')
        .velocity('fadeIn', {
          complete: () => {
            $(target.attr('data-status-indicator'))
              .velocity('fadeOut', { delay: 1000 });
          },
        });
    },

    revealEscapeChars (input) {
      const output = input.replace(/[<>&\n"]/g, x => ({
        '<': '&amp;lt;',
        '>': '&amp;gt;',
        '&': '&amp;&',
        '"': '&amp;quot;',
        '\n': '<br />',
      }[x]
      ));

      return output;
    },
  }
}
</script>
<style lang="scss" scoped></style>
