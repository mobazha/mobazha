<template>
  <div :class="`userPage clrS ${isBlockedUser ? 'isBlocked' : ''}`">
    <template v-if="!showPageNotFound && !showBlockedModal && !showUserLoading">
      <nav id="pageTabBar" class="barLg clrP clrBr">
        <div class="flexVCent pageTabs">
          <MiniProfile :options="{
            overwriteClickRating: true,
          }"
          :bb="function() {
            return {
              model: model,
            };
          }"
          @clickRating="clickRating" />
          <div class="flexExpand">
            <div class="flexHRight flexVCent gutterH clrT2">
              <DefineTabHeader v-slot="{tab, count}">
                <a :class="`btn tab clrBr ${activeTab === tab ? 'clrT active' : ''}`" @click="clickTab(tab)"
                >{{ ob.polyT(`userPage.mainNav.${tab}`) }}<span v-if="count !== null" class="clrTEmph1 margLSm">{{ abbrNum(count) }}</span></a>
              </DefineTabHeader>
              <ReuseTabHeader tab="home" />
              <!-- // the store tab is only visible to the user if they have vendor set to false -->
              <ReuseTabHeader v-if="ob.vendor || ob.ownPage" tab="store" :count="listingCount"></ReuseTabHeader>
              <ReuseTabHeader tab="following" :count="followingCount"></ReuseTabHeader>
              <ReuseTabHeader tab="followers" :count="followerCount"></ReuseTabHeader>
            </div>
          </div>
        </div>
      </nav>
      <div
        class="header js-header"
        :style="
          headerHash
            ? `background-image: url(${ob.getServerUrl(`ob/image/${headerHash}`)}), url('../imgs/defaultHeader.png')`
            : `background-image: url('../imgs/defaultHeader.png')`
        "
      >
        <div class="blockedOverlay clrP">
          <div class="flexCol flexHCent tx4">
            <i class="ion-eye-disabled tx1"></i>
            <div>{{ ob.polyT('userPage.blockedUserOverlayText') }}</div>
          </div>
        </div>
      </div>
      <div class="pageContent js-pageContent">
        <div class="pageControls flexVBase">
          <div class="flexExpand">
            <h1 class="txBg txUnb txUnl txGlow tabTitle js-tabTitle"></h1>
          </div>
          <div class="posR">
            <template v-if="ob.ownPage">
              <div class="btnStrip floR clrSh2">
                <a class="btn clrP clrBr" @click="clickCustomize">{{ ob.polyT('userPage.customize') }}</a>
                <a class="btn clrP clrBr" @click="clickCreateListing">{{ ob.polyT('userPage.createListing') }}</a>
                <!--
            <a class="btn clrP clrBr hide js-moreableBtn">{{ ob.polyT('userPage.block') }}</a>
            <a class="iconBtn clrP clrBr " @click="clickMore" ><i class="ion-android-more-vertical"></i> </a>
          -->
              </div>
            </template>

            <template v-else>
              <SocialBtns :options="{ targetID: model.id, }" />
            </template>
            <template v-if="ob.showStoreWelcomeCallout">
              <div class="storeWelcomeCallout js-storeWelcomeCallout arrowBoxBottom confirmBox clrP clrBr clrSh1 tx5">
                <div class="tx3 txB rowSm padSm">{{ ob.polyT('userPage.storeWelcomeCalloutTitle') }}</div>
                <hr class="clrBr rowMd" />
                <p class="rowMd">{{ ob.polyT('userPage.storeWelcomeCalloutBody') }}</p>
                <hr class="clrBr" />
                <div class="txCtr padSm">
                  <button class="btn clrP clrBr" @click="clickCloseStoreWelcomeCallout">
                    {{ ob.polyT('userPage.storeWelcomeCalloutBtnClose') }}
                  </button>
                </div>
              </div>
            </template>
          </div>
        </div>
        <div class="tabContent js-userPage-tabContent">
          <!-- insert the tab subview here -->
          <Home v-if="activeTab === 'home'" :bb="function() {
              return {
                model,
              }
            }" />
          <Store v-if="activeTab === 'store'"
            :bb="storeBB()"
            @listingsUpdate="onListingsUpdate"
            />
          <Follow v-if="activeTab === 'followers' || activeTab === 'following'" :key="activeTab"
            :options="{
              followType: activeTab,
              peerID: model.id,
            }"
            :bb="followBB(activeTab)"
            />
          <Reputation v-if="activeTab === 'reputation'" :bb="function() {
              return {
                model,
              }
            }" />
        </div>
      </div>
    </template>
    <PageNotFound v-else-if="showPageNotFound" />

    <Teleport to="#js-vueModal">
      <BlockedWarning v-if="showBlockedModal" :options="{ peerID }"
       @canceled="onBlockWarningCanceled"
       @unblock="onUnblock"
      />
      <Loading v-else-if="showUserLoading"
        :userName="_model.name"
        :userAvatarHashes="_model.avatarHashes"
        :contentText="loadingContextText"
        :isProcessing="isLoadingUser"
        @clickCancel="onClickLoadingCancel" @clickRetry="onClickLoadingRetry"/>
      <ListingDetail v-else-if="activeTab === 'store' && listing"
        :key="`${listing.cid}_${listingKey}`"
        :bb="function() {
          return {
            profile: model,
            model: listing,
          }
        }"
        @refresh="listingKey += 1"
        @close="onListingDetailClose"
      />
    </Teleport>
    
  </div>
</template>

<script>
import $ from 'jquery';
import { createReusableTemplate } from '@vueuse/core';

import app from '../../../backbone/app';
import { abbrNum } from '../../../backbone/utils';
import { isHiRez } from '../../../backbone/utils/responsive';
import { startAjaxEvent, endAjaxEvent, recordEvent } from '../../../backbone/utils/metrics';
import { launchEditListingModal, launchSettingsModal } from '../../../backbone/utils/modalManager';
import { isBlocked, isUnblocking, events as blockEvents } from '../../../backbone/utils/block';
import { isValidUserRoute }from '../../../backbone/utils/routeCheck'
import { getCurrentConnection } from '../../../backbone/utils/serverConnect';
import Listing from '../../../backbone/models/listing/Listing';
import Listings from '../../../backbone/collections/Listings';
import Followers from '../../../backbone/collections/Followers';

import BlockedWarning from '../modals/BlockedWarning.vue'
import Loading from '../modals/LoadingUser.vue'
import MiniProfile from '../MiniProfile.vue';
import PageNotFound from '../error-pages/PageNotFound.vue'

import Home from './Home.vue';
import Store from './Store.vue';
import Follow from './Follow.vue';
import Reputation from './Reputation.vue';
import ListingDetail from '../modals/listingDetail/Listing.vue';

const [DefineTabHeader, ReuseTabHeader] = createReusableTemplate();

const standardizedHash = (hash) => (hash.endsWith('/') ? hash.slice(0, hash.length - 1) : hash);

export default {
  components: {
    DefineTabHeader,
    ReuseTabHeader,

    PageNotFound,
    BlockedWarning,
    Loading,
    MiniProfile,
    Home,
    Store,
    Follow,
    Reputation,

    ListingDetail,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      handle: '',
      guild: '',
      activeTab: 'store',
      slug: '',
      showStoreWelcomeCallout: true,

      followingCount: 0,
      followerCount: 0,
      listingCount: 0,

      profileFetch: undefined,
      listingFetch: undefined,
      listing: undefined,
      showUserLoading: false,
      showPageNotFound: false,
      showBlockedModal: false,

      isBlockedUser: false,

      listingKey: 0,

      loadingContextText: '',
      isLoadingUser: false,
    };
  },
  watch: {
    $route() {
      // The app has been routed to a new route, let's
      // clean up by aborting all fetches
      if (this.profileFetch.abort) this.profileFetch.abort();
      if (this.listingFetch) this.listingFetch.abort();
    }
  },
  created() {
    this.initEventChain();

    let { guid, state, slug } = this.$route.params;
    if (this.$route.path === '/') {
      guid = app.profile.id;
    }

    this.init(guid, state, slug);
  },
  mounted() {
    this.setBlockedClass();

    const stats = this.model.get('stats');
    this.followingCount = stats.get('followingCount');
    this.followerCount = stats.get('followerCount');
    this.listingCount = stats.get('listingCount');

    this.curConn = getCurrentConnection();

    if (this.curConn && this.curConn.server) {
      this.showStoreWelcomeCallout = !this.curConn.server.get('dismissedStoreWelcome');
    }

    this.listenTo(app.ownFollowing, 'add', this.onOwnFollowingAdd);
    this.listenTo(app.ownFollowing, 'remove', this.onOwnFollowingRemove);

    this.listenTo(blockEvents, 'blocked unblocked', (data) => {
      if (data.peerIDs.includes(this.model.id)) {
        this.setBlockedClass();
      }
    });
  },
  unmounted() {
    if (this.followingFetch) this.followingFetch.abort();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        ownPage: this.ownPage,
        showStoreWelcomeCallout: this.showStoreWelcomeCallout,
      };
    },
    headerHash() {
      const headerHashes = this.model.get('headerHashes').toJSON();
      return headerHashes ? (isHiRez() ? headerHashes.large : headerHashes.medium) : '';
    },
    ownPage() {
      return this.model.id === app.profile.id;
    }
  },
  watch: {
  },
  methods: {
    abbrNum,
    onBlockWarningCanceled(){
      this.showBlockedModal = false;

      const prevHash = standardizedHash(app.router.prevHash);
      const locationHash = standardizedHash(location.hash);

      if (prevHash === locationHash) {
        // means there is no previous page - will go to our own node page
        app.router.navigate(`${app.profile.id}`, { replace: true, trigger: true, });
      } else {
        app.router.navigate(`${prevHash.slice(1)}`, { replace: true, trigger: true, });
      }
    },
    onUnblock() {
      this.showBlockedModal = false;

      let { guid, state, slug } = this.$route.params;

      this.init(guid, state, slug);
    },
    init(guid, state, slug) {
      this.listing = undefined;
      this.listingFetch = undefined;

      const options = this.preLoad(guid, state, slug);
      this.baseInit(options);

      if (options.showPageNotFound || options.showBlockedModal) {
        return;
      }

      this.loadData(guid, state, slug);
    },
    preLoad(guid, state, slug) {
      const pageState = state || 'store';

      if (!isValidUserRoute(guid, pageState, slug)) {
        return { showPageNotFound: true };
      }

      if (isBlocked(guid) && !isUnblocking(guid)) {
        return { showPageNotFound: false, showBlockedModal: true, peerID: guid };
      }

      return {
        activeTab: pageState,
        showBlockedModal: false,
        showPageNotFound: false,
      };
    },
    loadData(guid, state, slug) {
      this.showUserLoading = true;

      // Hack to pass the handle into this function, which should really only
      // happen when called from userViaHandle(). If a handle is being passed in,
      // it will be passed in as { handle: 'charlie' } as the first element of the
      // ...args argument.
      let handle;
      // if (args.length && args[0] && args[0].hasOwnProperty('handle')) {
      //   functionArgs = functionArgs.slice(1);
      //   handle = args[0].handle;
      // }

      startAjaxEvent('UserPageLoad');

      if (guid === app.profile.id) {
        // don't fetch our own profile, since we have it already
        this.profileFetch = $.Deferred().resolve();
      } else {
        this.profileFetch = this.model.fetch();
      }

      if (state === 'store') {
        if (slug) {
          this.listing = new Listing({
            slug,
          }, { guid });

          this.listingFetch = this.listing.fetch();
        }
      }

      let userPageFetchError = '';
      const profileFetch = this.profileFetch;
      const listingFetch = this.listingFetch;

      this.loadingContextText = app.polyglot.t('userPage.loading.loadingText', {
          name: `<b>${handle || `${guid.slice(0, 8)}…`}</b>`,
        }),
      this.isLoadingUser = true;

      $.whenAll(profileFetch, listingFetch).done(() => {
        handle = this.model.get('handle');
        if (handle) {
          app.router.cacheGuidHandle(guid, handle);
        }

        this.showUserLoading = false;

        // Setting the address bar which will ensure the most up to date handle (or none) is
        // shown in the address bar.
        app.router.setAddressBarText();

        if (this.activeTab === 'store' && !this.model.get('vendor') && guid !== app.profile.id) {
          // the user does not have an active store and this is not our own node
          if (state) {
            // You've explicitly tried to navigate to the store tab. Since it's not
            // available, we'll re-route to page-not-found
            this.showPageNotFound = true;
            return;
          }

          // You've attempted to find a user with no particular tab. Since store is not available
          // we'll take you to the home tab.
          app.router.navigate(`${guid}/home/${slug ? slug : ''}`, {trigger: true, replace: true});
          return;
        }

        if (!state) {
          app.router.navigate(`${guid}/store${slug ? '/'+slug : ''}`, {trigger: true, replace: true});
          // this.$router.replace(`${guid}/store/${slug ? slug : ''}`);
          return;
        }
      }).fail((...failArgs) => {
        const jqXhr = failArgs[0];
        const reason = (jqXhr && jqXhr.responseJSON && jqXhr.responseJSON.reason)
          || (jqXhr && jqXhr.responseText) || '';

        if (jqXhr === profileFetch && profileFetch.statusText === 'abort') return;
        if (jqXhr === listingFetch && listingFetch.statusText === 'abort') return;

        if (profileFetch.state() === 'rejected') {
          userPageFetchError = 'User Not Found';
        } else if (listingFetch.state() === 'rejected') {
          userPageFetchError = 'Listing Not Found';
        }

        userPageFetchError = userPageFetchError
          ? `${userPageFetchError} - ${reason || 'unknown'}`
          : reason || 'unknown';

        let contentText = app.polyglot.t('userPage.loading.failTextStore', {
          store: `<b>${handle || `${guid.slice(0, 8)}…`}</b>`,
        });

        if (profileFetch.state() === 'resolved' && listingFetch.state() === 'rejected') {
          const linkText = app.polyglot.t('userPage.loading.failTextListingLink');
          const listingSlug = slug.length > 25
            ? `${slug.slice(0, 25)}…` : slug;
          contentText = app.polyglot.t('userPage.loading.failTextListingWithLink', {
            listing: `<b>${listingSlug}</b>`,
            link: `<a href="#${guid}/store">${linkText}</a>`,
          });
        }

        this.loadingContextText = contentText;
        this.isLoadingUser = false;
      })
        .always(() => {
          const dismissedCallout = getCurrentConnection() && getCurrentConnection().server.get('dismissedDiscoverCallout');
          endAjaxEvent('UserPageLoad', {
            ownPage: guid === app.profile.id,
            tab: this.activeTab,
            dismissedCallout,
            listing: !!listingFetch,
            errors: userPageFetchError || 'none',
          });
        });
    },

    onClickLoadingCancel() {
      const prevHash = standardizedHash(app.router.prevHash);
      const locationHash = standardizedHash(location.hash);

      if (prevHash === locationHash) {
        // there is no previous page, let's navigate to our home page
        this.navigate(`${app.profile.id}`, { trigger: true, });
      } else {
        // go back to previous page
        window.history.back();
      }
    },

    onClickLoadingRetry() {
      let { guid, state, slug } = this.$route.params;

      this.init(guid, state, slug);
    },

    onOwnFollowingAdd(md) {
      if (this.ownPage) {
        this.followingCount += 1;
      } else if (md.id === this.model.id) {
        this.followerCount += 1;
      }
    },

    onOwnFollowingRemove(md) {
      if (this.ownPage) {
        this.followingCount -= 1;
      } else if (md.id === this.model.id) {
        this.followerCount -= 1;
      }
    },

    clickTab(tab) {
      recordEvent('UserPage_Tab', { tab });
      this.setTabState(tab);
    },

    clickMore() {
      $('.js-moreableBtn').toggleClass('hide');
    },

    clickCustomize() {
      recordEvent('Settings_Open', { origin: 'userPage' });
      launchSettingsModal({ initialTab: 'Page' });
    },

    clickCreateListing() {
      recordEvent('Listing_New', { origin: 'userPage' });
      const listingModel = new Listing({}, { guid: app.profile.id });

      launchEditListingModal({
        model: listingModel,
      });
    },

    clickCloseStoreWelcomeCallout() {
      recordEvent('UserPage_CloseStoreWelcome');
      if (this.curConn && this.curConn.server) {
        this.curConn.server.save({ dismissedStoreWelcome: true });

        this.showStoreWelcomeCallout = false;
      }
    },

    clickRating() {
      recordEvent('UserPage_ClickReputation');
      this.setTabState('reputation');
    },

    setBlockedClass() {
      this.isBlockedUser = isBlocked(this.model.id) && !isUnblocking(this.model.id);
    },

    followBB(type) {
      const collection = new Followers([], { peerID: this.model.id, type, });
      const model = this.model;

      return function() {
        return {
          collection,
          model,
        }
      };
    },

    storeBB() {
      let collection = new Listings([], { guid: this.model.id });
      let model = this.model;
      return function() {
        return {
          collection,
          model,
        }
      };
    },

    onListingsUpdate(listings) {
      this.listingCount = listings.length;
    },

    onListingDetailClose() {
      this.listing = null;

      const guid = this.model.id;
      app.router.navigate(`${guid}/store`, {trigger: false});
    },

    setTabState(state) {
      if (!state) {
        throw new Error('Please provide a state.');
      }

      this.activeTab = state;

      // // add tab to history
      const listingBaseUrl = this.model.get('handle') ? `@${this.model.get('handle')}` : this.model.id;
      app.router.navigateUser(`${listingBaseUrl}/${state}`, this.model.id);
    },
  },
};
</script>
<style lang="scss" scoped></style>
