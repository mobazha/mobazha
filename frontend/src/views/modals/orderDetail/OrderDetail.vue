<template>
  <div class="modal modalScrollPage tabbedModal orderDetail">
    <BaseModal :modalInfo="{ removeOnClose: true }">
      <template v-slot:component>
        <div class="topControls flex">
          <!-- // This is something found at the top of multiple modals. Would be nice to make this into a template
      // and componentize the css. -->
          <div v-if="returnText">
            <div class="btnStrip clrSh3">
              <a class="btn clrP clrBr clrT" @click="onClickReturnBox">
                <span class="ion-chevron-left margRSm"></span>
                {{ returnText }}
              </a>
            </div>
          </div>
        </div>

        <div class="flex gutterH">
          <div class="tabColumn gutterV">
            <div class="contentBox clrP clrBr clrSh3 tx4 featuredProfile js-featuredProfile"
              :disabled="isFetching || fetchFailed"></div>
            <div class="contentBox padMd clrP clrBr clrSh3" :disabled="isFetching || fetchFailed">
              <h1 class="h4 txUp clrT">{{ ob.polyT('tabMenuHeading') }}</h1>
              <div class="boxList tx4 clrTx1Br tabHeads">
                <a class="tab clrT row" @click="selectTab('summary')">{{ ob.polyT('orderDetail.navMenu.summary')
                }}</a>
                <a class="tab row" @click="selectTab('discussion')">
                  <span>{{ ob.polyT('orderDetail.navMenu.discussion') }}<span
                      class="unreadBadge discSm clrE1 clrBrEmph1 clrTOnEmph">{{
                        unreadChatMessagesText }}</span></span>
                </a>
                <a class=" tab row" @click="selectTab('contract')">{{ ob.polyT('orderDetail.navMenu.contract')
                }}</a>
              </div>
            </div>
            <div class="mainCtaWrap hide" :hidden="isFetching || fetchFailed">
              <ProcessingButton className="btn clrBAttGrad clrBrDec1 clrTOnEmph" btnText="Accept Order"/>
            </div>
            <div class="js-actionBarContainer"></div>
          </div>
          <div class="flexExpand posR">
            <div class="contentBox clrP clrBr clrSh3 mainContent">
              <div v-if="isFetching">
                <div class="center"><SpinnerSVG className="spinnerMd" /></div>
              </div>

              <div v-else-if="fetchFailed">
                <div class="center txCtr tx4">
                  <div :class="`txB ${ob.initialFetchErrorMessage ? 'rowTn' : 'row'}`">Unable to fetch order.</div>
                  <div v-if="fetchError">
                    <div class="row">{{ fetchError }}</div>
                  </div>
                  <a class="btn clrP clrBr clrSh2" @click="onClickRetryFetch">Retry</a>
                </div>
              </div>

              <div v-else>
                <section class="tabContent js-tabContent">
                  <Summary
                    v-if="_tab === 'summary'"
                    v-model="tabViewData"
                    ref="summaryTabView"
                    @clickFulfillOrder="selectTab('fulfillOrder')"
                    @clickResolveDispute="() => {
                      recordEvent('OrderDetails_DisputeResolveStart');
                      this.selectTab('resolveDispute');
                    }"
                    @clickDisputeOrder="() => {
                      this.recordDisputeStart();
                      this.selectTab('disputeOrder');
                    }"
                    @clickDiscussOrder="selectTab('discussion')"
                    />
                  <!-- insert the tab subview here -->
                </section>
              </div>
            </div>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import app from '../../../../backbone/app';
import { capitalize } from '../../../../backbone/utils/string';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import { getCurrencyByCode as getWalletCurByCode } from '../../../../backbone/data/walletCurrencies';
import {
  resolvingDispute,
  events as orderEvents,
} from '../../../../backbone/utils/order';
import { getCachedProfiles } from '../../../../backbone/models/profile/Profile';
import { recordEvent } from '../../../../backbone/utils/metrics';
import Case from '../../../../backbone/models/order/Case';
import OrderFulfillment from '../../../../backbone/models/order/orderFulfillment/OrderFulfillment';
import OrderDispute from '../../../../backbone/models/order/OrderDispute';
import ResolveDisputeMd from '../../../../backbone/models/order/ResolveDispute';
import ProfileBox from '../../../../backbone/views/modals/orderDetail/ProfileBox';
import Discussion from '../../../../backbone/views/modals/orderDetail/Discussion';
import ContractTab from '../../../../backbone/views/modals/orderDetail/contractTab/ContractTab';
import FulfillOrder from '../../../../backbone/views/modals/orderDetail/FulfillOrder';
import DisputeOrder from '../../../../backbone/views/modals/orderDetail/DisputeOrder';
import ResolveDispute from '../../../../backbone/views/modals/orderDetail/ResolveDispute';
import ActionBar from '../../../../backbone/views/modals/orderDetail/ActionBar';
import ContractMenuItem from '../../../../backbone/views/modals/orderDetail/ContractMenuItem';

import Summary from './summaryTab/Summary.vue';

import { toRaw } from 'vue';

export default {
  components: {
    Summary,
  },
  mixins: [],
  props: {
  },
  data () {
    return {
      isFetching: false,
      fetchFailed: false,
      fetchError: '',
      _tab: 'summary',

      tabViewData: {},

      model: {},
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$store.state.cart);
  },
  mounted () {
    this.render();
  },
  computed: {
    type () {
      return this.model instanceof Case ? 'case' : this.model.type;
    },
    activeTab () {
      return this._tab;
    },
    /**
     * Returns whether different action bar buttons should be displayed or not
     * based upon the order state.
     */
    actionBarButtonState () {
      const paymentCurData = this.model.paymentCoinData;

      return {
        showDisputeOrderButton:
          (!paymentCurData || !paymentCurData.supportsEscrowTimeout)
          && this.model.isOrderDisputable,
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
    }
  },
  methods: {

    loadData (opts = {}) {
      let transactions = toRaw(opts.transactions);
      console.log('transactions: ', transactions);
      this.model = transactions.model;
      this.returnText = app.polyglot.t(`transactions.${this.type}s.returnToFromOrder`),

      this.tabViewCache = {};

      if (!this.model) {
        throw new Error('Please provide an Order or Case model.');
      }

      this.listenTo(this.model, 'request', this.onOrderRequest);
      this.listenToOnce(this.model, 'sync', this.onFirstOrderSync);

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
        if (this.actionBar) {
          this.actionBar.setState(this.actionBarButtonState);
        }
      });

      this.listenTo(this.model, 'otherContractArrived', () => {
        if (this.contractMenuItem) {
          this.contractMenuItem.setState(this.contractMenuItemState);
        }
      });

      const socket = getSocket();

      if (socket) {
        this.listenTo(socket, 'message', this.onSocketMessage);
      }

      this.model.fetch();
    },

    events () {
      return {
        'click .js-toggleSendReceive': 'onClickToggleSendReceive',
      };
    },

    onOrderRequest (md, xhr) {
      this.isFetching = true;
      this.fetchError = '';
      this.fetchFailed = false;

      xhr.done(() => {
        this.isFetching = false;
        this.fetchFailed = false;
      }).fail((jqXhr) => {
        if (jqXhr.statusText === 'abort') return;

        if (jqXhr.responseJSON && jqXhr.responseJSON.reason) {
          this.fetchError = jqXhr.responseJSON.reason;
        }

        this.isFetching = false;
        this.fetchFailed = true;
      });
    },

    onFirstOrderSync () {
      this.stopListening(this.model, null, this.onOrderRequest);
      const featuredProfileState = { isFetching: false };
      let featuredProfileFetch;

      if (this.type === 'case') {
        if (this.model.get('buyerOpened')) {
          featuredProfileFetch = this.getBuyerProfile();
          featuredProfileState.peerID = this.model.buyerID;
          this.featuredProfilePeerID = this.model.buyerID;
        } else {
          featuredProfileFetch = this.getVendorProfile();
          featuredProfileState.peerID = this.model.vendorID;
          this.featuredProfilePeerID = this.model.vendorID;
        }
      } else if (this.type === 'sale') {
        featuredProfileFetch = this.getBuyerProfile();
        featuredProfileState.peerID = this.model.buyerID;
        this.featuredProfilePeerID = this.model.buyerID;
      } else {
        featuredProfileFetch = this.getVendorProfile();
        featuredProfileState.peerID = this.model.vendorID;
        this.featuredProfilePeerID = this.model.vendorID;
      }

      featuredProfileFetch.done((profile) => {
        this.featuredProfileMd = profile;
        if (this.featuredProfile) this.featuredProfile.setModel(this.featuredProfileMd);
      });

      if (this.featuredProfile) this.featuredProfile.setState(featuredProfileState);
    },

    onClickRetryFetch () {
      this.model.fetch();
    },

    onClickReturnBox () {
      this.close();
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

      if (e.jsonData.message
        && e.jsonData.message.subject === this.model.id
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
      console.log('selectTab: ', targ);
      if (!this[`create${capitalize(targ)}TabView`]) {
        throw new Error(`${targ} is not a valid tab.`);
      }

      this._tab = targ;
      let tabView = this.tabViewCache[targ];

      if (!this.currentTabView || this.currentTabView !== tabView) {
        $('.js-tab').removeClass('clrT active');
        $(`.js-tab[data-tab="${targ}"]`).addClass('clrT active');

        if (this.currentTabView) this.currentTabView.$el.detach();

        if (!tabView) {
          tabView = this[`create${capitalize(targ)}TabView`]();
          this.tabViewCache[targ] = tabView;
          tabView.render();
        }

        this._tabContent.append(tabView.$el);

        if (typeof tabView.onAttach === 'function') {
          tabView.onAttach.call(tabView);
        }

        if (tabView.autoFocusFirstField) {
          tabView.$el.find('select, input, textarea')[0].focus();
        }

        this.currentTabView = tabView;
      }
    },

    recordDisputeStart () {
      recordEvent('OrderDetails_DisputeStart', {
        type: this.type,
        state: { isFetching: this.isFetching, fetchError: this.fetchError, fetchFailed: this.fetchFailed },
      });
    },

    createSummaryTabView () {
      this.tabViewData = {
        model: this.model,
        vendor: {
          id: this.model.vendorID,
          getProfile: this.getVendorProfile.bind(this),
        },
        buyer: {
          id: this.model.buyerID,
          getProfile: this.getBuyerProfile.bind(this),
        },
      };

      if (this.model.moderatorID) {
        this.tabViewData.moderator = {
          id: this.model.moderatorID,
          getProfile: this.getModeratorProfile.bind(this),
        };
      }

      return this.$refs.summaryTabView;
    },

    createDiscussionTabView () {
      const amActiveTab = () => (this.activeTab === 'discussion');
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
        model: this.model,
        amActiveTab: amActiveTab.bind(this),
      };

      if (this.model.moderatorID) {
        this.tabViewData.moderator = {
          id: this.model.moderatorID,
          getProfile: this.getModeratorProfile.bind(this),
        };
      }

      const view = this.createChild(Discussion, this.tabViewData);
      this.listenTo(view, 'convoMarkedAsRead', () => {
        this.model.set('unreadChatMessages', 0);
        this.trigger('convoMarkedAsRead');
      });

      return view;
    },

    createContractTabView () {
      const view = this.createChild(ContractTab, {
        model: this.model,
      });

      this.listenTo(view, 'clickBackToSummary', () => this.selectTab('summary'));
      return view;
    },

    // This should not be called on a Case.
    createFulfillOrderTabView () {
      const contract = this.model.get('contract');

      const model = new OrderFulfillment(
        { orderID: this.model.id },
        {
          contractType: contract.type,
          isLocalPickup: contract.isLocalPickup,
        },
      );

      const view = this.createChild(FulfillOrder, {
        model,
        contractType: contract.type,
        isLocalPickup: contract.isLocalPickup,
      });

      this.listenTo(view, 'clickBackToSummary clickCancel', () => this.selectTab('summary'));

      return view;
    },

    createDisputeOrderTabView () {
      if (this.model.isCase) {
        throw new Error('This view should not be created on Cases.');
      }

      const contract = this.model.get('contract');
      const model = new OrderDispute({ orderID: this.model.id });
      const translationKeySuffix = app.profile.id === this.model.buyerID
        ? 'Buyer' : 'Vendor';
      let timeoutMessage = '';

      try {
        timeoutMessage = getWalletCurByCode(this.model.paymentCoin).supportsEscrowTimeout
          ? app.polyglot.t(
            `orderDetail.disputeOrderTab.timeoutMessage${translationKeySuffix}`,
            { timeoutAmount: contract.disputeExpiryVerbose },
          )
          : '';
      } catch (e) {
        // pass
      }

      const view = this.createChild(DisputeOrder, {
        model,
        contractType: contract.type,
        moderator: {
          id: this.model.moderatorID,
          getProfile: this.getModeratorProfile.bind(this),
        },
        timeoutMessage,
      });

      this.listenTo(view, 'clickBackToSummary clickCancel', () => this.selectTab('summary'));

      return view;
    },

    createResolveDisputeTabView () {
      let modelAttrs = { orderID: this.model.id };
      const isResolvingDispute = resolvingDispute(this.model.id);

      // If this order is in the process of the dispute being resolved, we'll
      // populate the model with the data that was posted to the server.
      if (isResolvingDispute) {
        modelAttrs = {
          ...modelAttrs,
          ...isResolvingDispute.data,
        };
      }

      const model = new ResolveDisputeMd(modelAttrs, {
        buyerContractArrived: () => !!this.model.get('buyerContract'),
        vendorContractArrived: () => !!this.model.get('vendorContract'),
        vendorProcessingError: () => this.model.vendorProcessingError,
      });

      const view = this.createChild(ResolveDispute, {
        model,
        case: this.model,
        vendor: {
          id: this.model.vendorID,
          getProfile: this.getVendorProfile.bind(this),
        },
        buyer: {
          id: this.model.buyerID,
          getProfile: this.getBuyerProfile.bind(this),
        },
      });

      this.listenTo(view, 'clickBackToSummary clickCancel', () => this.selectTab('summary'));

      return view;
    },

    render () {
      this._tabContent = $('.js-tabContent');

      if (this.featuredProfile) this.featuredProfile.remove();
      this.featuredProfile = this.createChild(ProfileBox, {
        model: this.featuredProfileMd || null,
        initialState: {
          isFetching: !this.featuredProfilePeerID,
          peerID: this.featuredProfilePeerID,
        },
      });
      $('.js-featuredProfile').html(this.featuredProfile.render().el);

      if (!this.isFetching && !this.fetchError) {
        this.selectTab(this.activeTab);

        if (this.actionBar) this.actionBar.remove();
        this.actionBar = this.createChild(ActionBar, {
          orderID: this.model.id,
          initialState: this.actionBarButtonState,
        });
        $('.js-actionBarContainer').html(this.actionBar.render().el);
        this.listenTo(this.actionBar, 'clickOpenDispute', () => {
          this.recordDisputeStart();
          this.selectTab('disputeOrder');
        });

        if (this.contractMenuItem) this.contractMenuItem.remove();
        this.contractMenuItem = this.createChild(ContractMenuItem, {
          initialState: {
            ...this.contractMenuItemState,
          },
        });
        this.getCachedEl('[data-tab="contract"]')
          .replaceWith(this.contractMenuItem.render().el);
      }

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
