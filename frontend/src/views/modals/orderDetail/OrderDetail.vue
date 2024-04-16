<template>
  <div class="modal modalScrollPage tabbedModal orderDetail">
    <BaseModal @close="close" >
      <template v-slot:component>
        <div class="topControls flex">
          <!-- // This is something found at the top of multiple modals. Would be nice to make this into a template
      // and componentize the css. -->
          <template v-if="ob.returnText">
            <div class="btnStrip clrSh3">
              <a class="btn clrP clrBr clrT" @click="close">
                <span class="ion-chevron-left margRSm"></span>
                {{ ob.returnText }}
              </a>
            </div>
          </template>
        </div>

        <div class="flex gutterH">
          <div class="tabColumn gutterV">
            <div class="contentBox clrP clrBr clrSh3 tx4 featuredProfile js-featuredProfile"
              :disabled="ob.isFetching || ob.fetchFailed">
              <ProfileBox v-if="featuredProfileMd"
                :options="{
                  isFetching: !featuredProfilePeerID,
                  peerID: featuredProfilePeerID,
                }"
                :bb="function() {
                  return {
                    model: featuredProfileMd,
                  };
                }"
              />
            </div>
            <div class="contentBox padMd clrP clrBr clrSh3" :disabled="ob.isFetching || ob.fetchFailed">
              <h1 class="h4 txUp clrT">{{ ob.polyT('tabMenuHeading') }}</h1>
              <div class="boxList tx4 clrTx1Br tabHeads">
                <a :class="`tab clrT row ${activeTab === 'summary' ? 'clrT active' : ''}`" @click="selectTab('summary')">{{ ob.polyT('orderDetail.navMenu.summary') }}</a>
                <a :class="`tab row ${activeTab === 'discussion' ? 'clrT active' : ''}`" @click="selectTab('discussion')">
                  <span>{{ ob.polyT('orderDetail.navMenu.discussion') }}<span class="unreadBadge discSm clrE1 clrBrEmph1 clrTOnEmph">{{ unreadChatMessagesText }}</span></span>
                </a>
                <ContractMenuItem ref="contractMenuItem"
                  :options="{
                    initialState: {
                      ...contractMenuItemState,
                    },
                  }"
                  :active="activeTab === 'contract'"
                  @click="selectTab('contract')"
                  />
              </div>
            </div>
            <div class="mainCtaWrap" v-show="!ob.isFetching && !ob.fetchFailed">
              <ProcessingButton className="btn clrBAttGrad clrBrDec1 clrTOnEmph" btnText="Accept Order"/>
            </div>
            <div class="js-actionBarContainer">
              <ActionBar v-if="!ob.isFetching && !ob.fetchError"
                ref="actionBar"
                :options="{
                  orderID: model.id,
                  initialState: actionBarButtonState,
                }"
                @clickOpenDispute="onClickOpenDispute" />
            </div>
          </div>
          <div class="flexExpand posR">
            <div class="contentBox clrP clrBr clrSh3 mainContent">
              <template v-if="ob.isFetching">
                <div class="center"><SpinnerSVG className="spinnerMd" /></div>
              </template>

              <template v-else-if="ob.fetchFailed">
                <div class="center txCtr tx4">
                  <div :class="`txB ${ob.initialFetchErrorMessage ? 'rowTn' : 'row'}`">Unable to fetch order.</div>
                  <template v-if="ob.fetchError">
                    <div class="row">{{ ob.fetchError }}</div>
                  </template>
                  <a class="btn clrP clrBr clrSh2" @click="onClickRetryFetch">Retry</a>
                </div>
              </template>

              <template v-else>
                <section class="tabContent">
                  <Summary
                    v-if="activeTab === 'summary'"
                    :key="model"
                    :options="tabViewData"
                    :bb="function() {
                      return {
                        model,
                      };
                    }"
                    @clickFulfillOrder="selectTab('fulfillOrder')"
                    @clickResolveDispute="() => {
                      recordEvent('OrderDetails_DisputeResolveStart');
                      selectTab('resolveDispute');
                    }"
                    @clickDisputeOrder="onClickOpenDispute"
                    @clickDiscussOrder="selectTab('discussion')"
                  />
                  <Discussion
                    v-if="activeTab === 'discussion'"
                    :options="tabViewData"
                    :bb="function() {
                      return {
                        model,
                      };
                    }"
                    @convoMarkedAsRead="() => {
                      model.set('unreadChatMessages', 0);
                      $emit('convoMarkedAsRead', model.id);
                    }"
                  />
                  <ContractTab
                    v-if="activeTab === 'contract'"
                    :bb="function() {
                      return {
                        model,
                      };
                    }"
                    @clickBackToSummary="() => {
                      selectTab('summary');
                    }"
                  />
                  <FulfillOrder
                    v-if="activeTab === 'fulfillOrder'"
                    :options="{
                      orderID: model.id,
                      contractType: model.get('contract').type,
                      isLocalPickup: model.get('contract').isLocalPickup,
                    }"
                    @clickBackToSummary="() => {
                      selectTab('summary');
                    }"
                    @clickCancel="() => {
                      selectTab('summary');
                    }"
                  />
                  <DisputeOrder
                    v-if="activeTab === 'disputeOrder'"
                    :options="disputeOrderOptions()"
                    @clickBackToSummary="() => {
                      selectTab('summary');
                    }"
                    @clickCancel="() => {
                      selectTab('summary');
                    }"
                  />
                  <ResolveDispute
                    v-if="activeTab === 'resolveDispute'"
                    :options="resolveDisputeOptions()"
                    :bb="function() {
                      return {
                        case: model,
                      };
                    }"
                    @clickBackToSummary="() => {
                      selectTab('summary');
                    }"
                    @clickCancel="() => {
                      selectTab('summary');
                    }"
                  />
                  <!-- insert the tab subview here -->
                </section>
              </template>
            </div>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../backbone/data/walletCurrencies';
import {
  resolvingDispute,
  events as orderEvents,
} from '../../../../backbone/utils/order';
import { getCachedProfiles } from '../../../../backbone/models/profile/Profile';
import { recordEvent } from '../../../../backbone/utils/metrics';
import Case from '../../../../backbone/models/order/Case';
import ActionBar from './ActionBar.vue'
import ContractMenuItem from './ContractMenuItem.vue'
import Summary from './summaryTab/Summary.vue';
import Discussion from './Discussion.vue';
import ContractTab from './contractTab/ContractTab.vue';
import FulfillOrder from './FulfillOrder.vue';
import DisputeOrder from './DisputeOrder.vue'
import ResolveDispute from './ResolveDispute.vue'
import ProfileBox from './ProfileBox.vue'

export default {
  components: {
    ActionBar,
    ContractMenuItem,
    Summary,
    Discussion,
    ContractTab,
    FulfillOrder,
    DisputeOrder,
    ResolveDispute,
    ProfileBox,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      _state: {
        isFetching: false,
        fetchFailed: false,
        fetchError: '',
      },
      activeTab: 'summary',

      featuredProfileMd: undefined,
      featuredProfilePeerID: '',

      tabViewData: {},
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
        ...this.model.toJSON(),
        returnText: this.options.returnText,
        type: this.type,
      };
    },
    type () {
      return this.model instanceof Case ? 'case' : this.model.type;
    },
    contract () {
      return this.model.get('contract');
    },
    /**
     * Returns whether different action bar buttons should be displayed or not
     * based upon the order state.
     */
    actionBarButtonState () {
      const paymentCurData = this.model.paymentCoinData;

      return {
        showDisputeOrderButton: (!paymentCurData || !paymentCurData.supportsEscrowTimeout) && this.model.isOrderDisputable,
      };
    },
    contractMenuItemState () {
      let tip = '';

      if (this.model.isCase && !this.model.bothContractsValid) {
        const buyerContractAvailableAndInvalid = this.model.get('buyerContract') && !this.model.isBuyerContractValid;
        const vendorContractAvailableAndInvalid = this.model.get('vendorContract') && !this.model.isVendorContractValid;

        if (buyerContractAvailableAndInvalid && vendorContractAvailableAndInvalid) {
          tip = app.polyglot.t('orderDetail.contractMenuItem.tipBothContractsHaveError');
        } else {
          // "contract" here means the contract we're guaranteed to have
          const isContractValid = this.model.get('buyerOpened')
            ? this.model.isBuyerContractValid : this.model.isVendorContractValid;
          const otherContract = this.model.get('buyerOpened')
            ? this.model.get('vendorContract') : this.model.get('buyerContract');
          const type = this.model.get('buyerOpened') ? 'Buyer' : 'Vendor';
          const otherType = this.model.get('buyerOpened') ? 'Vendor' : 'Buyer';

          if (!isContractValid) {
            tip = app.polyglot.t(`orderDetail.contractMenuItem.tip${type}ContractHasError`);
          }

          if (!otherContract) {
            tip += `${tip ? ' ' : ''}`
              + `${app.polyglot.t(`orderDetail.contractMenuItem.tip${otherType}ContractNotArrived`)}`;
          } else if (!this.model.isContractValid(!this.model.get('buyerOpened'))) {
            tip += `${tip ? ' ' : ''}`
              + `${app.polyglot.t(`orderDetail.contractMenuItem.tip${type}ContractHasError`)}`;
          }
        }
      }

      return { tip };
    },
    unreadChatMessagesText () {
      let count = this.model.get('unreadChatMessages');
      count = count > 0 ? count : '';
      count = count > 99 ? '…' : count;
      return count;
    },
  },
  methods: {
    recordEvent,
    loadData (options = {}) {
      const opts = {
        initialState: {
          isFetching: false,
          fetchFailed: false,
          fetchError: '',
        },
        initialTab: 'summary',
        ...options,
      };

      this.baseInit(opts);

      if (!this.model) {
        throw new Error('Please provide an Order or Case model.');
      }

      this.listenTo(this.model, 'request', this.onOrderRequest);
      this.listenToOnce(this.model, 'sync', this.onFirstOrderSync);
      // this.listenTo(this.model, 'change:unreadChatMessages', () => this.setUnreadChatMessagesBadge());

      this.listenTo(orderEvents, 'fulfillOrderComplete', () => {
        if (this.activeTab === 'fulfillOrder') this.selectTab('summary');
      });

      this.listenTo(orderEvents, 'openDisputeComplete', () => {
        if (this.activeTab === 'disputeOrder') this.selectTab('summary');
      });

      this.listenTo(orderEvents, 'resolveDisputeComplete', () => {
        if (this.activeTab === 'resolveDispute') this.selectTab('summary');
      });

      this.listenTo(this.model, 'change:state', () => {
        if (this.$refs.actionBar) {
          this.$refs.actionBar.setState(this.actionBarButtonState);
        }
      });

      this.listenTo(this.model, 'otherContractArrived', () => {
        if (this.$refs.contractMenuItem) {
          this.$refs.contractMenuItem.setState(this.contractMenuItemState);
        }
      });

      const socket = getSocket();

      if (socket) {
        this.listenTo(socket, 'message', this.onSocketMessage);
      }

      this.model.fetch().done(()=>{
        this.initTabViewData();
      });
    },

    onOrderRequest (md, xhr) {
      this.setState({
        isFetching: true,
        fetchError: '',
        fetchFailed: false,
      });

      xhr.done(() => {
        this.setState({
          isFetching: false,
          fetchFailed: false,
        });
      }).fail((jqXhr) => {
        if (jqXhr.statusText === 'abort') return;

        let fetchError = '';

        if (jqXhr.responseJSON && jqXhr.responseJSON.reason) {
          fetchError = jqXhr.responseJSON.reason;
        }

        this.setState({
          isFetching: false,
          fetchFailed: true,
          fetchError,
        });
      });
    },

    onFirstOrderSync () {
      this.stopListening(this.model, null, this.onOrderRequest);

      let featuredProfileFetch;

      if (this.type === 'case') {
        if (this.model.get('buyerOpened')) {
          featuredProfileFetch = this.getBuyerProfile();
          this.featuredProfilePeerID = this.model.buyerID;
        } else {
          featuredProfileFetch = this.getVendorProfile();
          this.featuredProfilePeerID = this.model.vendorID;
        }
      } else if (this.type === 'sale') {
        featuredProfileFetch = this.getBuyerProfile();
        this.featuredProfilePeerID = this.model.buyerID;
      } else {
        featuredProfileFetch = this.getVendorProfile();
        this.featuredProfilePeerID = this.model.vendorID;
      }

      featuredProfileFetch.done((profile) => {
        this.featuredProfileMd = profile;
      });
    },

    onClickRetryFetch () {
      this.model.fetch();
    },

    onSocketMessage (e) {
      const notificationTypes = [
        // A notification for the buyer that a payment has come in for the order. Let's refetch
        // our model so we have the data for the new transaction and can show it in the UI.
        // As of now, the buyer only gets these notifications and this is the only way to be
        // aware of partial payments in realtime.
        'payment',
        'newOrder',
        'orderPaymentReceived',
        // A notification the vendor will get when an offline order has been canceled
        'orderCancel',
        // A notification the vendor will get when an order has been fully funded
        'orderFunded',
        // A notification the buyer will get when the vendor has rejected an offline order.
        'orderDeclined',
        // A notification the buyer will get when the vendor has accepted an offline order.
        'orderConfirmation',
        // A notification the buyer will get when the vendor has refunded their order.
        'refund',
        // A notification the buyer will get when the vendor has fulfilled their order.
        'orderFulfillment',
        // A notification the vendor will get when the buyer has completed an order.
        'orderCompletion',
        // When a party opens a dispute the mod and the other party will get this notification
        'disputeOpen',

        'caseOpen',
        // Sent to the moderator when the other party (the one that didn't open the dispute) sends
        // their copy of the contract (which would occur if they were onffline when the dispute was
        // opened and have since come online).
        'caseUpdate',
        // Notification to the vendor and buyer when a mod has made a decision on an open dispute.
        'disputeClose',
        // Notification the other party will receive when a dispute payout is accepted (e.g. if vendor
        // accepts, the buyer will get this and vice versa).
        'disputeAccepted',
        // Socket received by buyer when the vendor has an error processing an offline order.
        'processingError',
        // Socket received by buyer then the vendor has released funds from escrow after the order
        // and/or dispute timed-out.
        'vendorFinalizedPayment',
      ];

      if (e.jsonData.notification && e.jsonData.notification.orderID === this.model.id) {
        if (notificationTypes.indexOf(e.jsonData.notification.type) > -1) {
          this.model.fetch();
        }
      }

      if (e.jsonData.chatMessage
        && e.jsonData.chatMessage.orderID === this.model.id
        && this.activeTab !== 'discussion') {
        const count = this.model.get('unreadChatMessages');
        this.model.set('unreadChatMessages', count + 1);
      }
    },

    _getParticipantProfile (participantType) {
      const peerID = this.model[`${participantType}ID`];
      const profileKey = `_${participantType}Profile`;

      if (!this[profileKey]) {
        if (peerID === app.profile.id) {
          const deferred = $.Deferred();
          deferred.resolve(app.profile);
          this[profileKey] = deferred.promise();
        } else {
          this[profileKey] = getCachedProfiles([peerID])[0];
        }
      }

      return this[profileKey];
    },

    /**
     * Returns a promise that resolves with the buyer's Profile model.
     */
    getBuyerProfile () {
      return this._getParticipantProfile('buyer');
    },

    /**
     * Returns a promise that resolves with the vendor's Profile model.
     */
    getVendorProfile () {
      return this._getParticipantProfile('vendor');
    },

    /**
     * Returns a promise that resolves with the moderator's Profile model.
     */
    getModeratorProfile () {
      return this._getParticipantProfile('moderator');
    },

    selectTab (targ) {
      this.activeTab = targ;
    },

    onClickOpenDispute() {
      this.recordDisputeStart();
      this.selectTab('disputeOrder');
    },

    recordDisputeStart () {
      recordEvent('OrderDetails_DisputeStart', {
        type: this.type,
        state: this.model.get('state'),
      });
    },

    initTabViewData () {
      this.tabViewData = {
        orderID: this.model.id,
        buyer: {
          id: this.model.buyerID,
          getProfile: this.getBuyerProfile.bind(this),
        },
        vendor: {
          id: this.model.vendorID,
          getProfile: this.getVendorProfile.bind(this),
        },
      };

      if (this.model.moderatorID) {
        this.tabViewData.moderator = {
          id: this.model.moderatorID,
          getProfile: this.getModeratorProfile.bind(this),
        };
      }
    },

    disputeOrderOptions () {
      const translationKeySuffix = app.profile.id === this.model.buyerID ? 'Buyer' : 'Vendor';
      let timeoutMessage = '';

      try {
        timeoutMessage = getWalletCurByCode(this.model.paymentCoin).supportsEscrowTimeout
          ? app.polyglot.t(
            `orderDetail.disputeOrderTab.timeoutMessage${translationKeySuffix}`,
            { timeoutAmount: this.contract.disputeExpiryVerbose },
          )
          : '';
      } catch (e) {
        // pass
      }

      return {
        orderID: this.model.id,
        contractType: this.contract.type,
        moderator: {
          id: this.model.moderatorID,
          getProfile: this.getModeratorProfile.bind(this),
        },
        timeoutMessage,
      }
    },
    resolveDisputeOptions () {
      return {
        vendor: {
          id: this.model.vendorID,
          getProfile: this.getVendorProfile.bind(this),
        },
        buyer: {
          id: this.model.buyerID,
          getProfile: this.getBuyerProfile.bind(this),
        },
      };
    },
  }
}
</script>
<style lang="scss" scoped></style>
