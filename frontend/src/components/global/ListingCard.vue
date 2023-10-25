<template>
  <div v-if="!cardError" @click.stop="onClick"
    :class="`listingCard col clrBr clrHover clrT clrP clrSh2 contentBox ${ownListing ? 'ownListing' : ''} ${destroyClass} ${blocked ? 'blocked' : ''} ${hideNsfw ? 'hideNsfw' : ''} ${_model.nsfw ? 'nsfw' : ''}`">
    <div v-if="ob.viewType === 'grid'" class="gridViewContent posR">
      <div class="listingImage" :style="listingImageStyle">
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
                  <a class="js-deleteConfirmCancel" @click.stop="onClickConfirmCancel">{{ ob.polyT('listingCard.confirmDelete.btnCancel') }}</a>
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
            <div class="js-blockBtnWrapper">
              <BlockBtn :options="{ targetID: ownerGuid, initialState: { useIcon: true }, }" />
            </div>

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
              :style="vendorAvatarStyle"
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
          <div class="listingImage posR" :style="listingImageStyle"></div>
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
          <div class="js-blockBtnWrapper">
            <BlockBtn :options="{ targetID: ownerGuid, initialState: { useIcon: true }, }" />
          </div>
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
import Report from '../../../backbone/views/modals/Report';

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
  unmounted() {
    if (this.fullListingFetch) this.fullListingFetch.abort();
    if (this.destroyRequest) this.destroyRequest.abort();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        ownListing: this.ownListing,
        coinType: this.model.get('currency') && this.model.get('currency').code,
        shipsFreeToMe: this.model.shipsFreeToMe,
        viewType: this.viewType,
        displayCurrency: app.settings.get('localCurrency'),
        isBlocked,
        isUnblocking,
        abbrNum,
      };
    },
    listingImageStyle() {
      const thumbnail = this.model.get('thumbnail');
      if (thumbnail) {
        let listingImageSrc = this.viewType === 'grid'
          ? app.getServerUrl(`ob/image/${isHiRez() ? thumbnail.medium : thumbnail.small}`)
          : app.getServerUrl(`ob/image/${isHiRez() ? thumbnail.small : thumbnail.tiny}`);

        return `background-image: url(${listingImageSrc}), url('../imgs/defaultItem.png')`;
      }

      return `background-image: url('../imgs/defaultItem.png')`;
    },
    vendorAvatarStyle() {
      const vendor = this.model.get('vendor');
      if (vendor && vendor.avatarHashes) {
        const avatarImageSrc = app.getServerUrl(`ob/image/${isHiRez() ? vendor.avatarHashes.small : vendor.avatarHashes.tiny}`);

        return `background-image: url(${avatarImageSrc}), url('../imgs/defaultAvatar.png')`;
      }
      return `background-image: url('../imgs/defaultAvatar.png')`;
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
    ownerGuid() {
      if (this.profile) {
        // If a profile model of the listing owner is available, please pass it in.
        return this.profile.id;
      } else if (this.model.get('vendor')) {
        // If a vendor object is available (part of proposed search API), please pass it in.
        return this.model.get('vendor').peerID;
      } else {
        // Otherwise please provide the store owner's guid.
        this.ownerGuid = this.options.ownerGuid;
      }
    },
    verifiedModID() {
      const moderators = this.model.get('moderators') || [];
      const verifiedIDs = app.verifiedMods.matched(moderators);
      return verifiedIDs[0];
    },
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
        // This should be initialized as null, so we could determine whether the user
        // never set this (null), or explicitly clicked to show / hide nsfw (true / false)
        this._userClickedShowNsfw = null;

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
          }
        });
      } catch (e) {
        this.cardError = e.message || true;

        if (this.showErrorCardOnError) {
          return;
        }

        throw e;
      }
    },

    attributes() {
      // make it possible to tab to this element
      return { tabIndex: 0 };
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


    onClick(e) {
      if (this.deleteConfirmOn) return;
      if (
        !this.ownListing ||
        (e.target !== $('.js-edit')[0] &&
          e.target !== $('.js-delete')[0] &&
          !$.contains($('.js-edit')[0], e.target) &&
          !$.contains($('.js-delete')[0], e.target))
      ) {
        const slug = this.model.get('slug');

        if (this.$route.params.guid === this.ownerGuid) {
          app.router.navigate(`${this.ownerGuid}/store/${slug}`);
        } else {
          app.router.navigate(`${this.ownerGuid}/store/${slug}`, { trigger: true });
        }
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
      this.$refs.reportBtn.setState({ reported: true });
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

    render(cardError = this.cardError) {
      let _cardError = cardError;

      if (!_cardError) {
        this.setBlockedClass();
        this.setHideNsfwClass();
      }

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
