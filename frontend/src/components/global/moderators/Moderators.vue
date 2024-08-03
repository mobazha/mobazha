<template>
  <div class="moderatorsList">
    <div :class="`moderatorsWrapper clrBr ${ob.wrapperClasses} js-moderatorsWrapper`">
      <!-- // placeholder so border collapse doesn't erase the table border when there are no table cells -->
      <template v-if="ob.placeholder">
        <div class="moderatorCard clrBr">
          <div class="moderatorCardInner"></div>
        </div>
      </template>
      <template v-if="ob.totalIDs && !ob.totalShown">
        <div class="moderatorCard moderatorsMessage clrBr">
          <div class="moderatorCardInner">
            <div class="flexCent">
              <div class="flexColRows flexHCent gutterVTn">
                <h4>{{ ob.polyT(`${msgPath}.title`, { coin: modCurrency }) }}</h4>
                <!-- // The section below is only relevant if the moderators are loaded in a purchasing context. -->
                <template v-if="ob.purchase">
                  <div class="tx4 clrT2">{{ ob.polyT(`${msgPath}.body`) }}</div>
                  <template v-if="hasHiddenUnverfied">
                    <!-- //just a spacer -->
                    <div class="padTn"></div>
                    <div>
                      <button class="btn clrP clrBr" @click="clickShowUnverified">{{ ob.polyT('moderators.showUnverified') }}</button>
                    </div>
                  </template>
                </template>
              </div>
            </div>
          </div>
        </div>
      </template>
      <template v-for="card in modCards" :key="`${card.model.id}_${showVerifiedOnly}_${modCurrency}`">
        <ModCard
          ref="modCards"
          v-show="modShouldRender(card.model)"
          :options="{
            purchase: innerOptions.purchase,
            notSelected: innerOptions.notSelected,
            radioStyle: innerOptions.radioStyle,
            controlsOnInvalid: innerOptions.controlsOnInvalid,
            initialState: {
              // Moderators that aren't being rendered should never be selected.
              selectedState: modShouldRender(card.model) ? innerOptions.cardState : innerOptions.notSelected,
              preferredCurs,
            },
          }"
          :bb="() => {
            return {
              model: card.model,
            }
          }"
          @modSelectChange="onModSelectChange" />
      </template>
    </div>
    <div class="js-statusWrapper">
      <!-- // create a moderators status view. It should retain it's state between renders of this view. -->
      <ModeratorsStatus
        ref="moderatorsStatus"
        v-show="showStatus"
        :options="{
          loaded: allIDs.length, // not shown if open fetch
          toLoad: fetchingMods.length, // not shown if open fetch
          total: modCards.length,

          initialState: {
            mode: method === 'GET' ? 'loaded' : 'loadingXofY',
            showLoadBtn,
            showSpinner,
          }
        }"
        @browseMore="onBrowseMore"
      />
    </div>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import bigNumber from 'bignumber.js';
import { guid } from '../../../../backbone/utils';
import app from '../../../../backbone/app';
import { myAjax } from '../../../api/api';
import { anySupportedByWallet } from '../../../../backbone/data/walletCurrencies';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import Moderators from '../../../../backbone/collections/Moderators';
import Profile from '../../../../backbone/models/profile/Profile';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import ModCard from './Card.vue';
import ModeratorsStatus from './Status.vue';

export default {
  components: {
    ModCard,
    ModeratorsStatus
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    showVerifiedOnly: {
      type: Boolean,
      default: false,
    },
    modCurrency: {
      type: String,
      default: '',
    },
    preferredCurs: {
      type: Object,
      default: [],
    }
  },
  data() {
    return {
      _state: {
        loading: false,
      },

      modCards: [],

      innerOptions: {
        apiPath: 'fetchprofiles',
        async: true,
        useCache: true,
        moderatorIDs: [],
        excludeIDs: [],
        method: 'POST',
        include: '',
        purchase: false,
        singleSelect: false,
        radioStyle: false,
        controlsOnInvalid: false,
        cardState: 'unselected',
        notSelected: 'unselected',
        showLoadBtn: false,
        showSpinner: true,
      },

      unVerifiedCount: 0,
      showStatus: false,

      fetchingMods: [],
      moderatorsCol: undefined,
      moderatorsColKey: 0,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    // render the moderators so it can start fetching and adding moderator cards
    if (this.moderatorIDs.length > 0) {
      this.getModeratorsByID();
    }
  },
  unmounted() {
    this.modFetches.forEach((fetch) => fetch.abort());
  },
  computed: {
    ob() {
      let access = this.showVerifiedOnly;

      const showMods = this.modCards.filter((card) => this.modShouldRender(card.model));
      this.unVerifiedCount = this.modCards.filter((card) => card.model.hasModCurrency(this.modCurrency) && !card.model.isVerified).length;
      const totalIDs = this.allIDs.length;

      return {
        ...this.templateHelpers,
        wrapperClasses: this.innerOptions.wrapperClasses,
        placeholder: !showMods.length && (this.unfetchedMods.length || !totalIDs),
        purchase: this.innerOptions.purchase,
        totalShown: showMods.length,
        totalIDs,
        ...this._state,
      };
    },
    hasHiddenUnverfied() {
      return this.showVerifiedOnly && this.unVerifiedCount;
    },
    msgPath() {
      return `moderators.noModsMsg.no${this.modCurrency ? 'Matching' : ''}${this.hasHiddenUnverfied ? 'Verified' : ''}Moderators`;
    },
    allIDs() {
      let access = this.moderatorsColKey;

      return this.moderatorsCol.pluck('peerID');
    },
    selectedIDs() {
      const IDs = [];
      (this.$refs.modCards ?? []).forEach((mod) => {
        if (mod.getState().selectedState === 'selected') {
          IDs.push(mod.model.id);
        }
      });
      return IDs;
    },
    unselectedIDs() {
      const IDs = [];
      (this.$refs.modCards ?? []).forEach((mod) => {
        if (mod.getState().selectedState !== 'selected') {
          IDs.push(mod.model.id);
        }
      });
      return IDs;
    },
  },
  methods: {
    /**
     * @param {string}  options.apiPath           - Current options are fetchprofiles and moderators.
     * @param {boolean} options.async             - Return profiles via websocket.
     * @param {boolean} options.useCache          - Use cached data for faster speed.
     * @param {array}   options.moderatorIDs      - list of moderators to retrieve. If none get all.
     * @param {array}   options.excludeIDs        - list of moderators to not use.
     * @param {string}  options.method            - POST or GET
     * @param {string}  options.include           - If apiPath is moderator, set to 'profile' or only
     *                                              the peerIDs of each moderator are returned.
     * @param {boolean} options.purchase          - If this is used in a purchase, pass this to the
     *                                              child moderator card views.
     * @param {boolean} options.singleSelect      - Allow only one moderator to be selected at a time.
     * @param {boolean} options.radioStyle        - Show the moderator cards with radio buttons.
     * @param {boolean} options.showInvalid       - Show invalid moderator cards.
     * @param {boolean} options.controlsOnInvalid - Show controls on invalid cards so they can be
     *                                              removed or otherwise acted on.
     * @param {string}  options.wrapperClasses    - Add classes to the card container.
     * @param {string}  options.fetchErrorTitle   - A title for the fetch error.
     * @param {string}  options.cardState         - The initial state for cards that are created.
     * @param {string}  options.notSelected       - Which not selected state to use on the mod cards.
     * @param {boolean} options.showLoadBtn       - Show the load more button in the status bar.
     * @param {boolean} options.showSpinner       - Show the spinner in the status bar
     */

    loadData(options = {}) {
      if (!options.fetchErrorTitle) {
        throw new Error('Please provide error text for the moderator fetch.');
      }

      const opts = {
        ...this.innerOptions,
        ...options,
        initialState: {
          loading: false,
          ...options.initialState,
        },
      };
      this.innerOptions = opts;

      if (!opts.apiPath || ['fetchprofiles', 'moderators'].indexOf(opts.apiPath) === -1) {
        throw new Error('The apiPath must be either fetchprofiles or moderators');
      }
      if (opts.apiPath === 'moderators') {
        if (opts.moderatorIDs.length) {
          throw new Error('If the apiPath is moderators, a list of IDs is not used.');
        }
      } else if (opts.apiPath === 'fetchprofiles' && opts.include) {
        throw new Error('If the apiPath is fetchprofiles, the include parameter is not used.');
      }
      if (typeof opts.async !== 'boolean') {
        throw new Error('The value of async must be a boolean');
      }
      if (!opts.fetchErrorTitle) {
        throw new Error('Please provide a title for the fetch error.');
      }

      if (opts.apiPath === 'moderators') {
        // this will fetch available moderators without POSTing a list of IDs.
        opts.include = 'profile';
        opts.method = 'GET';
      }

      this.baseInit(opts);
      this.unfetchedMods = [];
      this.fetchingMods = [];
      this.fetchingVerifiedMods = [];
      this.modFetches = [];
      this.moderatorsCol = new Moderators();
      this.listenTo(this.moderatorsCol, 'add', (model) => {
        this.moderatorsColKey += 1;

        this.addMod(model);
      });
      this.listenTo(this.moderatorsCol, 'remove', (md) => {
        this.moderatorsColKey += 1;

        const removeIndex = this.modCards.findIndex((card) => card.model === md);
        this.modCards.splice(removeIndex, 1);
      });
      this.modCards = [];

      // listen to the websocket for moderator data
      this.serverSocket = getSocket();
    },

    clickShowUnverified() {
      this.$emit('clickShowUnverified');
    },

    onBrowseMore() {
      this.getModeratorsByID();
    },

    removeNotFetched(ID) {
      this.unfetchedMods = this.unfetchedMods.filter((peerID) => peerID !== ID);
      this.checkNotFetched();
    },

    processMod(data) {
      // Don't add profiles that are not moderators unless showInvalid is true. The ID list may have
      // peerIDs that are out of date, and are no longer moderators.
      const isAMod = data.moderator && data.moderatorInfo;
      // If the moderator has an invalid currency, remove them from the list.
      // With multi-wallet, this should be a very rare occurrence.
      const modCurs = (data.moderatorInfo && data.moderatorInfo.acceptedCurrencies) || [];
      const supportedCur = anySupportedByWallet(modCurs);

      if (data.moderatorInfo && data.moderatorInfo.fee.feeType === 'FIXED_PLUS_PERCENTAGE' && !(data.moderatorInfo.fee.fixedFee.amount instanceof bigNumber)) {
        data.moderatorInfo.fee.fixedFee.amount = bigNumber(data.moderatorInfo.fee.fixedFee.amount);
      }

      if ((!!isAMod && supportedCur) || this.innerOptions.showInvalid) {
        const newMod = new Profile(data, { parse: true });
        if (newMod.isValid()) this.moderatorsCol.add(newMod);
        this.removeNotFetched(data.peerID);
      } else {
        // remove the invalid moderator from the notFetched list
        this.removeNotFetched(data.peerID);
      }
    },

    getModeratorsByID(opts) {
      const op = {
        ...this.innerOptions,
        ...opts,
      };

      if (!Array.isArray(op.moderatorIDs)) {
        throw new Error('Please provide the list of moderators as an array.');
      }

      // don't get any that have already been added or excluded, or the user's own id.
      const excluded = [app.profile.id, ...this.allIDs, ...this.excludeIDs];
      const IDs = _.without(op.moderatorIDs, excluded);
      const includeString = op.include ? `&include=${op.include}` : '';

      let urlString = `ob/${op.apiPath}?async=${op.async}${includeString}&usecache=${op.useCache}`;
      let asyncID;

      if (op.async) {
        asyncID = guid();
        urlString += `&asyncID=${asyncID}`;
      }

      const url = app.getServerUrl(urlString);

      this.unfetchedMods = IDs;
      this.fetchingMods = IDs;
      this.fetchingVerifiedMods = app.verifiedMods.matched(IDs);

      this.setState({
        loading: true,
        noValidModerators: false,
        noValidVerifiedModerators: !this.fetchingVerifiedMods.length,
      });

      // Either a list of IDs can be posted, or any available moderators can be retrieved with GET
      if (IDs.length || op.method === 'GET') {
        this.showStatus = true;
        this.$nextTick(() => {
          this.$refs.moderatorsStatus.setState({
            loading: true,
          });
        })

        if (op.async) {
          if (this.serverSocket) {
            this.listenTo(this.serverSocket, 'message', (event) => {
              const eventData = event.jsonData;
              if (eventData.error) {
                // errors don't have a message id, check to see if the peerID matches
                if (IDs.includes(eventData.peerID)) {
                  this.processMod(eventData);
                }
              } else if (eventData.id === asyncID && !excluded.includes(eventData.peerID)) {
                this.processMod(eventData.profile);
              }
            });
          } else {
            throw new Error('There is no connection to the server to listen to.');
          }
        }

        const fetch = myAjax({
          url,
          data: JSON.stringify(IDs),
          method: op.method,
        })
          .done((data) => {
            if (!op.async) {
              data.forEach((mod) => {
                if (!excluded.includes(mod.peerID)) this.processMod(mod.profile);
              });
              this.unfetchedMods = [];
              this.checkNotFetched();
              if (!data.length) this.$emit('noModsFound', { guids: IDs });
            }
          })
          .fail((xhr) => {
            if (xhr.statusText === 'abort') return;
            const failReason = xhr.responseJSON ? `\n\n${xhr.responseJSON.reason}` : '';
            const msg = `${op.fetchErrorMsg}${failReason}`;
            openSimpleMessage(op.fetchErrorTitle, msg);
          });
        this.modFetches.push(fetch);
      }
    },

    removeModeratorsByID(IDs) {
      if (!IDs) {
        throw new Error('You must provide the ID or IDs to remove.');
      }
      // Collect the models so they can be returned to the caller.
      const removed = [];
      IDs.forEach((id) => {
        removed.push(this.moderatorsCol.get(id));
      });
      this.moderatorsCol.remove(IDs);
      return removed;
    },

    checkNotFetched() {
      if (this.unfetchedMods.length === 0 && this.fetchingMods.length) {
        // All ids have been fetched and ids existed to fetch.
        if (this.$refs.moderatorsStatus) {
          // this.showStatus = false;
          this.$refs.moderatorsStatus.setState({
            loading: false,
          });
        }

        this.setState({ loading: false, });
      } else {
        // Either ids are still fetching, or this is an open fetch with no set ids.
        if (this.$refs.moderatorsStatus) {
          this.$refs.moderatorsStatus.setState({});
        }
      }
    },
    addMod(model) {
      if (!model || !(model instanceof Profile)) {
        throw new Error('Please provide a valid profile model.');
      }

      const modCard = {
        model,
      }

      // Add verified mods to the beginning.
      if (model.isVerified) {
        const firstUnverifiedIndex = this.modCards.findIndex((card) => !card.model.isVerified);
        const insertAtIndex = firstUnverifiedIndex < 0 ? 0 : firstUnverifiedIndex;
        this.modCards.splice(insertAtIndex, 0, modCard);
      } else {
        this.modCards.push(modCard);
      }
    },

    onModSelectChange(data) {
      if (data.selected) {
        // If only one moderator should be selected, deselect the other moderators.
        if (this.innerOptions.singleSelect) this.deselectOthers(data.guid);
        this.$emit('cardSelect');
      }
    },

    modShouldRender(model) {
      const hideOnUnverified = this.showVerifiedOnly && !model.isVerified;
      const hideOnCur = this.modCurrency && !model.hasModCurrency(this.modCurrency);
      return !(hideOnUnverified || hideOnCur);
    },

    deselectMod(peerID) {
      if (!peerID) throw new Error('You must provide a peerID.');

      const mod = this.$refs.modCards.filter((card) => card.model.get('peerID') === peerID);
      if (mod.length) mod[0].changeSelectState(this.innerOptions.notSelected);
    },
    deselectOthers(peerID = '') {
      this.$refs.modCards.forEach((card) => {
        if (card.model.get('peerID') !== peerID) {
          card.changeSelectState(this.innerOptions.notSelected);
        }
      });
    },
  },
};
</script>
<style lang="scss" scoped></style>
