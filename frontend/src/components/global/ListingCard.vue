<template>
  <div v-if="!cardError" @click.stop="onClick"
    :class="`listingCard col clrBr clrHover clrT clrP clrSh2 contentBox ${ownListing ? 'ownListing' : ''} ${destroyClass} ${blocked ? 'blocked' : ''} ${hideNsfw ? 'hideNsfw' : ''} ${_model.nsfw ? 'nsfw' : ''}`">
    <div v-if="ob.viewType === 'grid'" class="gridViewContent posR">
      <div class="listingImage js-listingImage" :style="listingImageBgStyle">
        <div class="nsfwOverlay overlayPanel coverFull clrP">
          <div class="flexCent">
            <div>
              <div class="flexCol flexHCent gutterV">
                <div class="flexHCent gutterHSm tx3">
                  <div v-html="ob.parseEmojis('😲')" />
                  <div v-html="ob.parseEmojis('😱')" />
                </div>
                <button class="btn clrP clrBr tx6 clrSh1" @click.stop="onClickShowNsfw">{{ ob.polyT('listingCard.btnShowMatureContent') }}</button>
              </div>
            </div>
          </div>
        </div>
        <div class="blockedOverlay overlayPanel coverFull clrP">
          <div class="flexCent">
            <div>
              <div class="flexCol flexHCent">
                <i class="ion-eye-disabled tx1"></i>
                <div>{{ ob.polyT('listingCard.blockedUser') }}</div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="ob.ownListing" class="editOverlay overlayPanel">
          <div class="overlayPanelInner"></div>
          <div class="flex gutterHSm">
            <button
              @click.stop="onClickEdit"
              class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-edit"
              :data-tip="ob.polyT('listingCard.editListingTooltip')"
            >
              <span class="ion-edit"></span>
            </button>
            <button
              @click.stop="onClickClone"
              class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-clone"
              :data-tip="ob.polyT('listingCard.cloneListingTooltip')"
            >
              <span class="ion-ios-copy"></span>
            </button>
            <div class="posR">
              <a
                @click.stop="onClickDelete"
                class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-delete"
                :data-tip="ob.polyT('listingCard.deleteListingTooltip')"
                ><span class="ion-trash-b"></span
              ></a>
              <div v-show="deleteConfirmOn" class="js-deleteConfirmedBox confirmBox deleteConfirm tx5 arrowBoxBottom clrBr clrP clrT hide" @click.stop="onClickDeleteConfirmBox">
                <div class="tx3 txB rowSm">{{ ob.polyT('listingCard.confirmDelete.title') }}</div>
                <p>{{ ob.polyT('listingCard.confirmDelete.body') }}</p>
                <hr class="clrBr row" />
                <div class="flexHRight flexVCent gutterHLg buttonBar">
                  <a class="js-deleteConfirmCancel" @click="onClickConfirmCancel">{{ ob.polyT('listingCard.confirmDelete.btnCancel') }}</a>
                  <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph js-deleteConfirmed" @click.stop="onClickConfirmedDelete">{{ ob.polyT('listingCard.confirmDelete.btnConfirm') }}</a>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="additionalOverlay overlayPanel">
          <div class="flex gutterHSm">
            <div class="hideIfEmpty js-reportBtnWrapper">
              <ReportBtn v-if="reportsUrl" @startReport="startReport"/>
            </div>
            <div class="js-blockBtnWrapper"></div>

            <button
              v-if="ob.nsfw"
              class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop btnHideNsfw"
              @click.stop="onClickHideNsfw"
              :data-tip="ob.polyT('listingCard.tipHideMatureContent')"
            >
              <i class="ion-locked"></i>
            </button>
          </div>
        </div>
      </div>
      <div class="pad clrBr borderTop infoArea">
        <template v-if="ob.vendor">
          <a class="userIconWrapper js-userLink" :href="`#${ob.vendor.peerID}/store`" @click.stop="onClickUserLink">
            <div
              class="userIcon disc clrBr2 clrSh1 toolTipNoWrap js-vendorIcon"
              :style="`background-image: ` + (ob.vendorAvatarImageSrc ? `url(${ob.vendorAvatarImageSrc}), ` : '') +`url('../imgs/defaultAvatar.png')`"
              :data-tip="ob.vendor.name"
            ></div>
          </a>
          <div class="userIconWrapper nsfwAvatarOverlay">
            <div class="userIcon disc clrBr2 clrSh1 clrP tx3">
              <div v-html="ob.parseEmojis('😲')" />
            </div>
          </div>
          <div class="userIconWrapper blockedAvatarOverlay">
            <div class="userIcon disc clrBr2 clrSh1 clrP">
              <i class="ion-eye-disabled"></i>
            </div>
          </div>
        </template>

        <div class="rowSm">
          <!-- // The accepted currencies check is in case that data is not provided (e.g. a search provider omits),
      // we'll fall back to displaying the title field, since otherwise we're unable to determine one of
      // the trading pairs. -->
          <div
            v-if="ob.contractType !== 'CRYPTOCURRENCY' || !(ob.acceptedCurrencies && ob.acceptedCurrencies.length)"
            :class="`${ob.title.length > 60 ? 'toolTip' : 'toolTipNoWrap'} toolTipTop inlineBlock ${ob.vendor ? 'trimWidth' : ''}`"
            :data-tip="ob.title"
          >
            <a class="clrT clamp listingTitle">{{ ob.title }}</a>
          </div>
          <div v-else>
            <a class="listingTitle"
              v-html="ob.crypto.tradingPair({
                className: 'cryptoTradingPairSm',
                fromCur: `${ob.acceptedCurrencies && ob.acceptedCurrencies[0]}`,
                toCur: ob.price.currencyCode,
                truncateCurAfter: 5,
              })">
            </a>
          </div>
        </div>

        <div :class="`flexVCent ${priceRowTextClass}`">
          <div class="flexExpand ratingStrip" v-html="ob.formatRating(ob.averageRating, ob.ratingCount)"></div>

          <div v-if="ob.contractType !== 'CRYPTOCURRENCY'"
            v-html="ob.currencyMod.convertAndFormatCurrency(ob.price.amount, ob.price.currencyCode, ob.displayCurrency, {
                maxDisplayDecimals: priceMaxDisplayDecimals,
              })">
          </div>
          <div v-else
            v-html="ob.crypto.cryptoPrice({
              priceAmount: ob.price.amount,
              priceCurrencyCode: ob.price.currencyCode,
              displayCurrency: ob.displayCurrency,
              priceModifier: ob.price.modifier,
              wrappingClass: '',
              marketRelativityClass: 'hide',
              convertAndFormatOpts: {
                maxDisplayDecimals: priceMaxDisplayDecimals,
              },
            })">
          </div>
        </div>
      </div>
      <div class="listingIcons">
        <span v-if="ob.shipsFreeToMe" class="clrE1 clrTOnEmph phraseBox">{{ ob.polyT('listingCard.freeShippingBanner') }}</span>
      </div>
      <div class="verifiedModWrapper js-verifiedMod">
        <VerifiedMod :options="getListingOptions({
            model: verifiedModID && app.verifiedMods.get(verifiedModID),
          })"/>
      </div>
    </div>

    <div v-else-if="ob.viewType === 'list'" class="listViewContent">
      <div class="flexVCent gutterHSm">
        <!-- // Since we have inconsistent padding/gutters, we'll inline some padding settings. -->
        <div class="flexNoShrink posR">
          <div class="listingImage js-listingImage posR" :style="listingImageBgStyle"></div>
          <div class="center tx2 nsfwAvatarOverlay"><div v-html="ob.parseEmojis('😲')" /></div>
        </div>
        <div class="flexExpand">
          <template v-if="ob.contractType !== 'CRYPTOCURRENCY' || !Array.isArray(ob.acceptedCurrencies) || !ob.acceptedCurrencies.length">
            <div :class="`rowTn inlineBlock ${ob.title.length > 60 ? 'toolTip' : 'toolTipNoWrap'} toolTipTop`" :data-tip="ob.title">
              <a class="clrT clamp3 listingTitle">{{ ob.title }}</a>
            </div>
          </template>
          <div v-else v-html="ob.crypto.tradingPair({
            className: 'cryptoTradingPairSm',
            fromCur: ob.acceptedCurrencies[0],
            toCur: ob.price.currencyCode,
            truncateCurAfter: 5,
          })">
          </div>
          <div class="flexVCent gutterHSm tx5b">
            <div class="flexNoShrink ratingStrip" v-html="ob.formatRating(ob.averageRating, ob.ratingCount)">
            </div>
            <div class="verifiedModWrapper js-verifiedMod">
              <VerifiedMod :options="getListingOptions({
                model: verifiedModID && app.verifiedMods.get(verifiedModID),
              })"/>
            </div>
          </div>
        </div>

        <div v-if="ob.shipsFreeToMe" class="freeShipCol flexNoShrink txCtr">
          <span class="clrE1 clrTOnEmph clamp4 phraseBox">{{ ob.polyT('listingCard.freeShippingBanner') }}</span>
        </div>

        <div class="priceCol flexNoShrink clamp4">
          <span
            v-html="ob.contractType !== 'CRYPTOCURRENCY'
              ? ob.currencyMod.convertAndFormatCurrency(ob.price.amount, ob.price.currencyCode, ob.displayCurrency, {
                  maxDisplayDecimals: priceMaxDisplayDecimals,
                })
              : ob.crypto.cryptoPrice({
                  priceAmount: ob.price.amount,
                  priceCurrencyCode: ob.price.currencyCode,
                  displayCurrency: ob.displayCurrency,
                  priceModifier: ob.price.modifier,
                  wrappingClass: '',
                  convertAndFormatOpts: {
                    maxDisplayDecimals: priceMaxDisplayDecimals,
                  },
                })"
          >
          </span>
        </div>
      </div>
    </div>

    <div v-else class="flexVCent gutterH">
      <div class="tradeFromCol">
        <div class="flexVCent gutterHSm">
          <CryptoIcon :code="`${(ob.acceptedCurrencies && ob.acceptedCurrencies[0]) || ''}`" className="flexNoShrink" />
          <span class="txB clamp">{{ (ob.acceptedCurrencies && ob.acceptedCurrencies[0]) || '' }}</span>
        </div>
      </div>
      <div class="tradeArrowCol">
        <span class="pairingSeparator clrT2 ion-android-arrow-forward"></span>
      </div>
      <div class="tradeToCol">
        <div class="flexVCent gutterHSm">
          <CryptoIcon :code="ob.coinType" />
          <span class="txB clamp">{{ ob.coinType }}</span>
        </div>
      </div>
      <div class="vendorCol">
        <div class="flex gutterH">
          <div>
            <a class="userIcon disc clrBr2 clrSh1 js-userLink" :style="ob.getAvatarBgImage(ob.vendor.avatarHashes)" :href="`#${ob.vendor.peerID}/store`" @click.stop="onClickUserLink"></a>
          </div>
          <div class="flexCol gutterVTn">
            <div>
              <a class="clrT clamp js-userLink" :href="`#${ob.vendor.peerID}/store`" @click.top="onClickUserLink">{{ ob.vendor.name }}</a>
            </div>

            <div class="cl2amp clrT2 tx6 flexVCent gutterHSm">
              <div class="ratingStrip"
                v-html="ob.vendor && ob.vendor.stats ? ob.formatRating(averageStoreRating, totalStoreRatings) : ob.formatRating(0, 0)">
              </div>
              <div class="verifiedModWrapper">
                <div class="js-verifiedMod">
                  <VerifiedMod :options="getListingOptions({
                    model: verifiedModID && app.verifiedMods.get(verifiedModID),
                  })"/>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div
        class="priceCol"
        v-html="ob.crypto.cryptoPrice({
          priceAmount: ob.price.amount,
          priceCurrencyCode: ob.price.currencyCode,
          displayCurrency: ob.displayCurrency,
          priceModifier: ob.price.modifier,
          wrappingClass: 'txB',
        })">
      </div>

      <!-- // This is being commented out until inventory is functional.
    <div class="inventoryCol flexExpand flexVCent flexHRight gutterHSm {{ inventoryTxClass }}">{{ inventory }}</div> -->
    </div>

    <template v-if="['list', 'cryptoList'].includes(ob.viewType)">
      <div class="blockedOverlay overlayPanel coverFull clrP">
        <div class="flexCent">
          <div>
            <div class="flexCol flexHCent">
              <i class="ion-eye-disabled tx4"></i>
              <div>{{ ob.polyT('listingCard.blockedUser') }}</div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="ob.ownListing" class="editOverlay overlayPanel clrP">
        <div class="overlayPanelInner"></div>
        <div class="flexHCent gutterHSm">
          <button
            @click.stop="onClickEdit"
            class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-edit"
            :data-tip="ob.polyT('listingCard.editListingTooltip')"
          >
            <span class="ion-edit"></span>
          </button>
          <button
            @click.stop="onClickClone"
            class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-clone"
            :data-tip="ob.polyT('listingCard.cloneListingTooltip')"
          >
            <span class="ion-ios-copy"></span>
          </button>
          <div class="posR">
            <button
              @click.stop="onClickDelete"
              class="iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-delete"
              :data-tip="ob.polyT('listingCard.deleteListingTooltip')"
            >
              <span class="ion-trash-b"></span>
            </button>
            <div v-show="deleteConfirmOn" class="js-deleteConfirmedBox confirmBox deleteConfirm tx5 arrowBoxBottom clrBr clrP clrT hide" @click.stop="onClickDeleteConfirmBox">
              <div class="tx3 txB rowSm">{{ ob.polyT('listingCard.confirmDelete.title') }}</div>
              <p>{{ ob.polyT('listingCard.confirmDelete.body') }}</p>
              <hr class="clrBr row" />
              <div class="flexHRight flexVCent gutterHLg buttonBar">
                <a class="js-deleteConfirmCancel" @click="onClickConfirmCancel">{{ ob.polyT('listingCard.confirmDelete.btnCancel') }}</a>
                <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph js-deleteConfirmed" @click.stop="onClickConfirmedDelete">{{ ob.polyT('listingCard.confirmDelete.btnConfirm') }}</a>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="additionalOverlay overlayPanel clrP">
        <div class="flex gutterHSm">
          <div class="hideIfEmpty js-reportBtnWrapper">
            <ReportBtn v-if="reportsUrl" @startReport="startReport"/>
          </div>
          <div class="js-blockBtnWrapper"></div>
          <button
            class="btn clrP clrBr iconBtnSm btnShowNsfw clrSh1 toolTipNoWrap toolTipTop"
            @click.stop="onClickShowNsfw"
            tabindex="0"
            :data-tip="ob.polyT('listingCard.tipShowMatureContent')"
          >
            <i class="ion-unlocked"></i>
          </button>

          <button
            v-if="ob.nsfw"
            class="btn clrP clrBr iconBtnSm btnHideNsfw clrSh1 toolTipNoWrap toolTipTop"
            @click.stop="onClickHideNsfw"
            tabindex="0"
            :data-tip="ob.polyT('listingCard.tipHideMatureContent')"
          >
            <i class="ion-locked"></i>
          </button>
        </div>
      </div>
    </template>

    <div class="deleteOverlay coverFull overlayPanel">
      <div class="overlayPanelInner clrS"></div>
      <div class="deletingText clrT tx5">{{ ob.polyT('listingCard.deleting') }}</div>
      <div class="deletedText clrT tx5">
        <div class="ion-trash-b tx3"></div>
        {{ ob.polyT('listingCard.deleted') }}
      </div>
    </div>
  </div>
  <div v-else class="listingCard col clrBr clrT clrP clrSh2 contentBox ListingCard-errorCard" v-html="cardErrorInfo">
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import $ from 'jquery';
import app from '../../../backbone/app';
import { abbrNum } from '../../../backbone/utils';
import { launchEditListingModal } from '../../../backbone/utils/modalManager';
import { isBlocked, isUnblocking, events as blockEvents } from '../../../backbone/utils/block';
import { isHiRez } from '../../../backbone/utils/responsive';
import { startAjaxEvent, endAjaxEvent, recordEvent } from '../../../backbone/utils/metrics';
import { getNewerHash, outdateHash } from '../../../backbone/utils/outdatedListingHashes';
import Listing from '../../../backbone/models/listing/Listing';
import ListingShort from '../../../backbone/models/listing/ListingShort';
import { events as listingEvents } from '../../../backbone/models/listing';
import { openSimpleMessage } from '../../../backbone/views/modals/SimpleMessage';
import ListingDetail from '../../../backbone/views/modals/listingDetail/Listing';
import Report from '../../../backbone/views/modals/Report';
import BlockedWarning from '../../../backbone/views/modals/BlockedWarning';
import ReportBtn from '../../../backbone/views/components/ReportBtn';
import BlockBtn from '../../../backbone/views/components/BlockBtn';
import UserLoadingModal from '../../../backbone/views/userPage/Loading';

import { getListingOptions } from '@/utils/verifiedMod'

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      cardError: false,
      showErrorCardOnError: true,

      destroyClass: '',
      blocked: false,
      hideNsfw: false,

      avatarImage: undefined,

      app: app,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.render();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._model,
        ownListing: this.ownListing,
        coinType: this.model.get('currency') && this.model.get('currency').code,
        shipsFreeToMe: this.model.shipsFreeToMe,
        viewType: this.viewType,
        displayCurrency: app.settings.get('localCurrency'),
        isBlocked,
        isUnblocking,
        listingImageSrc: (this.listingImage.loaded && this.listingImage.src) || '',
        vendorAvatarImageSrc: (this.avatarImage && this.avatarImage.loaded && this.avatarImage.src) || '',
        abbrNum,
      };
    },
    listingImageBgStyle() {
      let ob = this.ob;
      const listingImageSrc = ob.listingImageSrc ? `url(${ob.listingImageSrc}), ` : '';
      return `background-image: ${listingImageSrc}url('../imgs/defaultItem.png')`;
    },
    priceMaxDisplayDecimals() {
      let ob = this.ob;
      if (!ob.currencyMod.isFiatCur(ob.displayCurrency)) return 6;
    },
    ownListing() {
      return app.profile.id === this.ownerGuid;
    },
    cardErrorInfo() {
      let messageHtml = app.polyglot.t('listingCard.cardError');

      if (typeof this.cardError === 'string') {
        messageHtml += `&nbsp;<span class="toolTip" data-tip="${this.cardError}">` + '<span class="ion-help-circled clrTErr"></span></span>';
      }

      return `<p class="padMd clrTErr tx5">${messageHtml}</p>`;
    },
    priceRowTextClass() {
      let priceRowTextClass = '';

      try {
        const formattedRating = this.ob.formatRating(ob.averageRating, ob.ratingCount);
        const priceLength = this.ob.price.amount.toFormat().length;
        const ratingLength = ($(`<div>${formattedRating}</div>`).text()).length;

        if (priceLength + ratingLength > 17) {
          priceRowTextClass = 'txBase'
        }
      } catch (e) {
        // pass
      }

      return priceRowTextClass;
    },
    verifiedModID() {
      const moderators = this.model.get('moderators') || [];
      const verifiedIDs = app.verifiedMods.matched(moderators);
      return verifiedIDs[0];
    }
  },
  methods: {
    getListingOptions,
    loadData(options = {}) {
      const opts = {
        viewType: 'grid',
        reportsUrl: '',
        searchUrl: '',
        ...options,
      };

      this.baseInit(opts);

      this.cardError = false;

      try {
        if (!this.model || !(this.model instanceof ListingShort)) {
          throw new Error('Please provide a ListingShort model.');
        }

        // Any provided profile model or vendor info object will also be passed into the
        // listing detail modal.
        if (opts.profile) {
          // If a profile model of the listing owner is available, please pass it in.
          this.ownerGuid = opts.profile.id;
        } else if (this.model.get('vendor')) {
          // If a vendor object is available (part of proposed search API), please pass it in.
          this.ownerGuid = this.model.get('vendor').peerID;
        } else {
          // Otherwise please provide the store owner's guid.
          this.ownerGuid = opts.ownerGuid;
        }

        if (typeof this.ownerGuid === 'undefined') {
          throw new Error('Unable to determine ownership of the listing. Please either provide' + ' a profile model or pass in an ownerGuid option.');
        }

        if (!opts.listingBaseUrl) {
          // When the listing card is clicked and the listing detail modal is
          // opened, the slug of the listing is concatenated with the listingBaseUrl
          // and the route is updated (both history & address bar).
          throw new Error('Please provide a listingBaseUrl.');
        }

        if (this.ownListing) {
          this.listenTo(listingEvents, 'destroying', (md, destroyingOpts) => {
            if (this.isRemoved()) return;

            if (destroyingOpts.slug === this.model.get('slug')) {
              this.destroyClass = 'listingDeleting';
            }

            destroyingOpts.xhr.fail(() => this.destroyClass = '');
          });

          this.listenTo(listingEvents, 'destroy', (md, destroyOpts) => {
            if (this.isRemoved()) return;

            if (destroyOpts.slug === this.model.get('slug')) {
              this.destroyClass = 'listingDeleted';
            }
          });
        }

        this.viewType = opts.viewType;
        this.reportsUrl = opts.reportsUrl;
        this.deleteConfirmOn = false;
        this.boundDocClick = this.onDocumentClick.bind(this);
        // This should be initialized as null, so we could determine whether the user
        // never set this (null), or explicitly clicked to show / hide nsfw (true / false)
        this._userClickedShowNsfw = null;
        $(document).on('click', this.boundDocClick);

        this.listenTo(blockEvents, 'blocked unblocked', (data) => {
          if (data.peerIDs.includes(this.ownerGuid)) {
            this.setBlockedClass();
          }
        });

        this.listenTo(app.settings, 'change:showNsfw', () => {
          this._userClickedShowNsfw = null;
          this.setHideNsfwClass();
        });

        this.verifiedMods = app.verifiedMods.matched(this.model.get('moderators'));

        this.listenTo(app.verifiedMods, 'update', () => {
          const newVerifiedMods = app.verifiedMods.matched(this.model.get('moderators'));
          if ((this.verifiedMods.length && !newVerifiedMods.length) || (!this.verifiedMods.length && newVerifiedMods.length)) {
            this.verifiedMods = newVerifiedMods;
            this.render();
          }
        });

        // load necessary images in a cancelable way
        const thumbnail = this.model.get('thumbnail');
        const listingImageSrc =
          this.viewType === 'grid'
            ? app.getServerUrl(`ob/image/${isHiRez() ? thumbnail.medium : thumbnail.small}`)
            : app.getServerUrl(`ob/image/${isHiRez() ? thumbnail.small : thumbnail.tiny}`);

        this.listingImage = new Image();
        this.listingImage.addEventListener('load', () => {
          this.listingImage.loaded = true;
          $('.js-listingImage').css('backgroundImage', `url(${listingImageSrc})`);
        });
        this.listingImage.src = listingImageSrc;

        const vendor = this.model.get('vendor');
        if (vendor && vendor.avatarHashes) {
          const avatarImageSrc = app.getServerUrl(`ob/image/${isHiRez() ? vendor.avatarHashes.small : vendor.avatarHashes.tiny}`);

          this.avatarImage = new Image();
          this.avatarImage.addEventListener('load', () => {
            this.avatarImage.loaded = true;
            $('.js-vendorIcon').css('backgroundImage', `url(${avatarImageSrc})`);
          });
          this.avatarImage.src = avatarImageSrc;
        }
      } catch (e) {
        this.cardError = e.message || true;

        if (this.showErrorCardOnError) {
          this.render();
          return;
        }

        throw e;
      }
    },

    attributes() {
      // make it possible to tab to this element
      return { tabIndex: 0 };
    },
    onDocumentClick() {
      this.deleteConfirmOn = false;
    },
    onClickEdit() {
      recordEvent('Lisitng_EditFromCard');
      app.loadingModal.open();

      this.fetchFullListing()
        .done((xhr) => {
          if (xhr.statusText === 'abort' || this.isRemoved()) return;

          launchEditListingModal({
            model: this.getFullListing(),
          });
        })
        .always(() => {
          if (this.isRemoved()) return;
          app.loadingModal.close();
        });
    },

    onClickDelete() {
      recordEvent('Lisitng_DeleteFromCard');
      this.deleteConfirmOn = true;
    },

    onClickClone() {
      recordEvent('Lisitng_CloneFromCard');
      app.loadingModal.open();

      this.fetchFullListing()
        .done((xhr) => {
          if (xhr.statusText === 'abort' || this.isRemoved()) return;
          launchEditListingModal({
            model: this.getFullListing().cloneListing(),
          });
        })
        .always(() => {
          if (this.isRemoved()) return;
          app.loadingModal.close();
        });
    },

    onClickConfirmedDelete() {
      recordEvent('Lisitng_DeleteFromCardConfirm');
      if (this.destroyRequest && this.destroyRequest.state === 'pending') return;
      this.destroyRequest = this.model.destroy({ wait: true });
    },

    onClickConfirmCancel() {
      recordEvent('Lisitng_DeleteFromCardCancel');
      this.deleteConfirmOn = false;
    },

    onClickDeleteConfirmBox() {
    },

    onClickUserLink() {
    },

    loadListingDetail(hash = this.model.get('cid')) {
      const routeOnOpen = location.hash.slice(1);
      app.router.navigateUser(`${this.options.listingBaseUrl}${this.model.get('slug')}`, this.ownerGuid);

      startAjaxEvent('Listing_LoadFromCard');
      const segmentation = {
        ownListing: !!this.ownListing,
        openedFromStore: !!this.options.onStore,
        searchUrl: (this.options.searchUrl && this.options.searchUrl.hostname) || 'none',
      };

      let storeName = `${this.ownerGuid.slice(0, 8)}…`;
      let avatarHashes;
      let title = this.model.get('title');
      title = title.length > 25 ? `${title.slice(0, 25)}…` : title;

      if (this.options.profile) {
        storeName = this.options.profile.get('name');
        avatarHashes = this.options.profile.get('avatarHashes').toJSON();
      } else if (this.options.vendor) {
        storeName = this.options.vendor.name;
        avatarHashes = this.options.vendor.avatarHashes;
      }

      if (storeName.length > 40) {
        storeName = `${storeName.slice(0, 40)}…`;
      }

      let ipnsFetch = (this.ipnsFetch = null);
      let ipfsFetch = (this.ipfsFetch = null);

      const onFailedListingFetch = (xhr) => {
        if (typeof xhr !== 'object') {
          throw new Error('Please provide the failed xhr.');
        }

        this.userLoadingModal.setState({
          contentText: app.polyglot.t('userPage.loading.failTextListing', {
            listing: `<b>${title}</b>`,
          }),
          isProcessing: false,
        });

        let err = (xhr.responseJSON && xhr.responseJSON.reason) || xhr.statusText || 'unknown error';
        // Consolidate and remove specific data from no link errors.
        if (err.startsWith('no link named')) err = 'no link named under hash';
        endAjaxEvent('Listing_LoadFromCard', {
          ...segmentation,
          errors: err,
        });
      };

      const showListingDetail = () => {
        endAjaxEvent('Listing_LoadFromCard', {
          ...segmentation,
        });

        const listingDetail = new ListingDetail({
          model: this.getFullListing(),
          profile: this.options.profile,
          vendor: this.options.vendor,
          closeButtonClass: 'cornerTR iconBtn clrP clrBr clrSh3 toolTipNoWrap',
          modelContentClass: 'modalContent',
          openedFromStore: !!this.options.onStore,
          checkNsfw: !this._userClickedShowNsfw,
        })
          .render()
          .open();

        const onListingDetailClose = () => {
          app.router.navigate(routeOnOpen);
          if (ipfsFetch) ipfsFetch.abort();
          ipnsFetch.abort();
        };

        listingDetail.purchaseModal.progress((getPurchaseE) => {
          if (getPurchaseE.type === ListingDetail.PURCHASE_MODAL_CREATE) {
            const purchaseModal = getPurchaseE.view;
            this.listenTo(purchaseModal, 'clickReloadOutdated', (e) => {
              e.preventDefault();
              listingDetail.render();
              purchaseModal.remove();
            });
          }
        });

        this.listenTo(listingDetail, 'close', onListingDetailClose);
        this.listenTo(listingDetail, 'modal-will-remove', () => this.stopListening(null, null, onListingDetailClose));
        this.listenTo(listingDetail, 'clickReloadOutdated', (e) => {
          // Since the model will already have been updated by
          // handleOutdated, we could just re-render here.
          listingDetail.render();
          e.preventDefault();
        });

        this.$emit('listingDetailOpened');
        this.userLoadingModal.remove();
        app.loadingModal.close();
      };

      const handleOutdatedHash = (listingData = {}, hashData) => {
        const { oldHash, newHash } = hashData;

        if (typeof listingData !== 'object') {
          throw new Error('Please provide the listing data as an object.');
        }

        if (typeof oldHash !== 'string' || !oldHash) {
          throw new Error('Please provide an oldHash as a non-empty string.');
        }

        if (typeof newHash !== 'string' || !newHash) {
          throw new Error('Please provide an newHash as a non-empty string.');
        }

        recordEvent('Lisitng_OutdatedHashFromCard', segmentation);

        this.getFullListing().set(this.getFullListing().parse(listingData));

        // push mapping to outdatedHashes collection
        outdateHash(oldHash, newHash);
      };

      const loadListing = () => {
        const listingHash = getNewerHash(hash || this.model.get('hash'));

        if (listingHash && this.ownerGuid !== app.profile.id) {
          ipfsFetch = this.getFullListing().fetch({
            hash: listingHash,
            showErrorOnFetchFail: false,
          });
          ipnsFetch = $.ajax(Listing.getIpnsUrl(this.ownerGuid, this.model.get('slug')));
        } else {
          ipnsFetch = this.getFullListing().fetch({ showErrorOnFetchFail: false });
        }

        if (this.userLoadingModal) this.userLoadingModal.remove();
        this.userLoadingModal = new UserLoadingModal({
          initialState: {
            userName: avatarHashes ? storeName : undefined,
            userAvatarHashes: avatarHashes,
            contentText: app.polyglot.t('userPage.loading.loadingText', {
              name: `<b>${title}</b>`,
            }),
            isProcessing: true,
          },
        });

        this.listenTo(this.userLoadingModal, 'clickCancel', () => {
          ipnsFetch.abort();
          if (ipfsFetch) ipfsFetch.abort();
          this.userLoadingModal.remove();
          app.router.navigate(routeOnOpen);
        });

        this.listenTo(this.userLoadingModal, 'clickRetry', () => {
          app.router.navigate(routeOnOpen);
          this.loadListingDetail(hash);
        });

        this.userLoadingModal.render().open();

        ipnsFetch
          .done((data, textStatus, xhr) => {
            if (xhr.statusText === 'abort' || this.isRemoved()) return;

            if (ipfsFetch && ['pending', 'rejected'].includes(ipfsFetch.state())) {
              ipfsFetch.abort();
              this.getFullListing().set(this.getFullListing().parse(data));
            }

            if (ipfsFetch && ipfsFetch.state() === 'resolved') {
              if (listingHash !== data.cid) {
                handleOutdatedHash(data, {
                  oldHash: listingHash,
                  newHash: data.cid,
                });
              }
            } else {
              showListingDetail();
            }
          })
          .fail((xhr) => {
            if (xhr.statusText === 'abort') return;

            if (ipfsFetch && ['pending', 'resolved'].includes(ipfsFetch.state())) return;

            onFailedListingFetch(xhr);
          });

        if (ipfsFetch) {
          ipfsFetch
            .done((data, textStatus, xhr) => {
              if (xhr.statusText === 'abort' || this.isRemoved()) return;
              showListingDetail();
            })
            .fail((xhr) => {
              if (xhr.statusText === 'abort') return;
              onFailedListingFetch(xhr);
            });
        }
      };

      if (isBlocked(this.ownerGuid) && !isUnblocking(this.ownerGuid)) {
        const blockedWarningModal = new BlockedWarning({ peerID: this.ownerGuid }).render().open();

        this.listenTo(blockedWarningModal, 'canceled', () => {
          app.router.navigate(routeOnOpen);
        });

        const onUnblock = () => loadListing();

        this.listenTo(blockEvents, 'unblocking unblocked', onUnblock);

        this.listenTo(blockedWarningModal, 'close', () => {
          this.stopListening(null, null, onUnblock);
        });
      } else {
        loadListing();
      }
    },

    onClick(e) {
      if (this.deleteConfirmOn) return;
      if (
        !this.ownListing ||
        (e.target !== $('.js-edit')[0] &&
          e.target !== $('.js-delete')[0] &&
          !$.contains($('.js-edit')[0], e.target) &&
          !$.contains($('.js-delete')[0], e.target))
      ) {
        this.loadListingDetail();
      }
    },

    onClickShowNsfw() {
      this._userClickedShowNsfw = true;
      this.setHideNsfwClass();
    },

    onClickHideNsfw(e) {
      this._userClickedShowNsfw = false;
      this.setHideNsfwClass();
    },

    setBlockedClass() {
      this.blocked = isBlocked(this.ownerGuid);
    },

    setHideNsfwClass() {
      // explicitly checking for false, since null means something different
      this.hideNsfw = this._userClickedShowNsfw === false || (this.model.get('nsfw') && !this._userClickedShowNsfw && !app.settings.get('showNsfw'));
    },

    fetchFullListing(options = {}) {
      const opts = {
        showErrorOnFetchFail: true,
        ...options,
      };

      if (this.fullListingFetch && this.fullListingFetch.state() === 'pending') {
        return this.fullListingFetch;
      }

      this.fullListingFetch = this.getFullListing()
        .fetch()
        .fail((xhr) => {
          if (!opts.showErrorOnFetchFail || xhr.statusText === 'abort' || this.isRemoved()) return;
          let failReason = (xhr.responseJSON && xhr.responseJSON.reason) || '';

          if (xhr.status === 404) {
            failReason = app.polyglot.t('listingCard.editFetchErrorDialog.bodyNotFound');
          }

          openSimpleMessage(app.polyglot.t('listingCard.editFetchErrorDialog.title'), failReason);
        });

      return this.fullListingFetch;
    },

    onReportSubmitted() {
      this.reportBtn.setState({ reported: true });
    },

    startReport() {
      if (this.report) this.report.remove();

      this.report = this.createChild(Report, {
        removeOnClose: true,
        peerID: this.ownerGuid,
        slug: this.model.get('slug'),
        url: this.reportsUrl,
      })
        .render()
        .open();

      this.report.on('modal-will-remove', () => {
        this.report = null;
      });
      this.listenTo(this.report, 'submitted', this.onReportSubmitted);
    },

    getFullListing() {
      if (!this.fullListing) {
        this.fullListing = new Listing(
          {
            slug: this.model.get('slug'),
          },
          {
            guid: this.ownerGuid,
          }
        );
      }
      return this.fullListing;
    },

    remove() {
      if (this.listingImage) this.listingImage.src = '';
      if (this.avatarImage) this.avatarImage.src = '';
      if (this.fullListingFetch) this.fullListingFetch.abort();
      if (this.destroyRequest) this.destroyRequest.abort();
      $(document).off('click', this.boundDocClick);
      if (this.userLoadingModal) this.userLoadingModal.remove();
      if (this.ipnsFetch) this.ipnsFetch.abort();
      if (this.ipfsFetch) this.ipfsFetch.abort();
    },
    render(cardError = this.cardError) {
      let _cardError = cardError;

      if (!_cardError) {
        this.setBlockedClass();
        this.setHideNsfwClass();

        if (!this.ownListing) {
          this.getCachedEl('.js-blockBtnWrapper').html(
            new BlockBtn({
              targetId: this.ownerGuid,
              initialState: { useIcon: true },
            }).render().el
          );
        }
      }

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
