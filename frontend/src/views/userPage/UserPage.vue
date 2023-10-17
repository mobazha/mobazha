<template>
  <div :class="`userPage clrS ${isBlockedUser ? 'isBlocked' : ''}`">
    <template v-if="!showPageNotFound && !showBlockedModal">
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
              <a class="btn tab clrBr js-tab" @click="clickTab" data-tab="home">{{ ob.polyT('userPage.mainNav.home') }}</a>
              <!-- // the store tab is only visible to the user if they have vendor set to false -->
              <a v-if="ob.vendor || ob.ownPage" class="btn tab clrBr js-tab" @click="clickTab" data-tab="store">
                {{ ob.polyT('userPage.mainNav.store') }}<span class="clrTEmph1 margLSm js-listingsCount">{{ ob.stats.listingCount }}</span></a>
              <a class="btn tab clrBr js-tab" @click="clickTab" data-tab="following">
                {{ ob.polyT('userPage.mainNav.following') }}<span class="clrTEmph1 margLSm">{{ abbrNum(followingCount) }}</span></a>
              <a class="btn tab clrBr js-tab" @click="clickTab" data-tab="followers">
                {{ ob.polyT('userPage.mainNav.followers') }}<span class="clrTEmph1 margLSm">{{ abbrNum(followerCount) }}</span></a>
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
        </div>
      </div>
    </template>
    <PageNotFound v-else-if="showPageNotFound" />

    <Teleport to="#js-vueModal">
      <BlockedWarning v-if="showBlockedModal" :options="{ peerID }"
       @canceled="onBlockWarningCanceled"
       @close="cleanUpBlockedModal"
      />
      <Loading v-else-if="showUserLoading" :contentText="loadingContextText" :isProcessing="isLoadingUser"
        @clickCancel="onClickLoadingCancel" @clickRetry="onClickLoadingRetry"/>
    </Teleport>
    
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../backbone/app';
import { followsYou } from '../../../backbone/utils/follow';
import { abbrNum } from '../../../backbone/utils';
import { capitalize } from '../../../backbone/utils/string';
import { isHiRez } from '../../../backbone/utils/responsive';
import { startAjaxEvent, endAjaxEvent, recordEvent } from '../../../backbone/utils/metrics';
import { launchEditListingModal, launchSettingsModal } from '../../../backbone/utils/modalManager';
import { isBlocked, isUnblocking, events as blockEvents } from '../../../backbone/utils/block';
import { isValidUserRoute }from '../../../backbone/utils/routeCheck'
import { getCurrentConnection } from '../../../backbone/utils/serverConnect';
import Listing from '../../../backbone/models/listing/Listing';
import Listings from '../../../backbone/collections/Listings';
import Followers from '../../../backbone/collections/Followers';
import Home from '../../../backbone/views/userPage/Home';
import Store from '../../../backbone/views/userPage/Store';
import Follow from '../../../backbone/views/userPage/Follow';
import Reputation from '../../../backbone/views/userPage/Reputation';

import BlockedWarning from '../modals/BlockedWarning.vue'
import Loading from './Loading.vue'
import MiniProfile from '../MiniProfile.vue';
// import Home from './Home.vue'
import PageNotFound from '../error-pages/PageNotFound.vue'

export default {
  components: {
    PageNotFound,
    BlockedWarning,
    Loading,
    MiniProfile,
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
      state: 'store',
      slug: '',
      showStoreWelcomeCallout: true,

      followingCount: 0,
      followerCount: 0,

      tabViewCache: {},
      tabViews: { Home, Store, Follow, Reputation },

      profileFetch: undefined,
      listingFetch: undefined,
      showUserLoading: false,
      showPageNotFound: false,
      showBlockedModal: false,

      isBlockedUser: false,

      loadingContextText: '',
      isLoadingUser: true,
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
  beforeRouteUpdate(to) {
    let { guid, state, slug } = to.params;
    this.init(guid, state, slug);
  },
  mounted() {
    this.render();
  },
  unmounted() {
    if (this.followingFetch) this.followingFetch.abort();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._model,
        ownPage: this.ownPage,
        showStoreWelcomeCallout: this.showStoreWelcomeCallout,
      };
    },
    headerHash() {
      const headerHashes = this._model.headerHashes;
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
      if (window.history.state.back === null) {
        // there is no previous page, let's navigate to our home page
        this.$router.push({ path: `/${app.profile.id}`});
      } else {
        // go back to previous page
        this.$router.back();
      }
    },
    onUnblock(data) {
      if (data.peerIDs.includes(guid)) {
        app.loadingModal.open();
        this.user(guid, state, ...args);
      }
    },
    init(guid, state, slug) {
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

      let profileFetch;
      let listing;
      let listingFetch;

      startAjaxEvent('UserPageLoad');

      if (guid === app.profile.id) {
        // don't fetch our own profile, since we have it already
        profileFetch = $.Deferred().resolve();
      } else {
        profileFetch = this.model.fetch();
      }

      if (state === 'store') {
        if (slug) {
          listing = new Listing({
            slug,
          }, { guid });

          listingFetch = listing.fetch();
        }
      }

      return {
        state: pageState,
        profileFetch,
        listing,
        listingFetch,
        showUserLoading: true,
        showBlockedModal: false,
        showPageNotFound: false,
      };
    },
    loadData(guid, state, slug) {
      // Hack to pass the handle into this function, which should really only
      // happen when called from userViaHandle(). If a handle is being passed in,
      // it will be passed in as { handle: 'charlie' } as the first element of the
      // ...args argument.
      let handle;
      // if (args.length && args[0] && args[0].hasOwnProperty('handle')) {
      //   functionArgs = functionArgs.slice(1);
      //   handle = args[0].handle;
      // }

      let userPageFetchError = '';
      const profileFetch = this.profileFetch;
      const listingFetch = this.listingFetch;
      $.whenAll(profileFetch, listingFetch).done(() => {
        handle = this.model.get('handle');
        if (handle) {
          app.router.cacheGuidHandle(guid, handle);
        }

        this.showUserLoading = false;

        // Setting the address bar which will ensure the most up to date handle (or none) is
        // shown in the address bar.
        app.router.setAddressBarText();

        if (this.state === 'store' && !this.model.get('vendor') && guid !== app.profile.id) {
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
          app.router.navigate(`${guid}/store/${slug ? slug : ''}`, {trigger: true, replace: true});
          // this.$router.replace(`${guid}/store/${slug ? slug : ''}`);
          return;
        }
        this.showUserLoading = false;

        this.initUserPage();
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
          const dismissedCallout = getCurrentConnection()
            && getCurrentConnection().server.get('dismissedDiscoverCallout');
          endAjaxEvent('UserPageLoad', {
            ownPage: guid === app.profile.id,
            tab: this.state,
            dismissedCallout,
            listing: !!listingFetch,
            errors: userPageFetchError || 'none',
          });
        });
    },
    initUserPage() {
      this.setBlockedClass();

      const stats = this.model.get('stats');
      this.followingCount = stats.get('followingCount');
      this.followerCount = stats.get('followerCount');

      if (!this.ownPage) {
        if (this.followerCount === 0 && app.ownFollowing.indexOf(this.model.id) > -1) {
          this.followerCount = 1;
        }
      } else {
        this.followingCount = app.ownFollowing.length;
      }

      this.curConn = getCurrentConnection();

      if (this.curConn && this.curConn.server) {
        this.showStoreWelcomeCallout = !this.curConn.server.get('dismissedStoreWelcome');
      }

      this.listenTo(app.ownFollowing, 'add', this.onOwnFollowingAdd);
      this.listenTo(app.ownFollowing, 'remove', this.onOwnFollowingRemove);

      this.followsYou = false;
      followsYou(this.model.id).done((data) => {
        if (this.miniProfile) {
          this.miniProfile.setState({ followsYou: data.followsMe });
        }

        if (this.followingCount === 0 && !this.ownPage) this.followingCount = 1;
      });

      this.listenTo(blockEvents, 'blocked unblocked', (data) => {
        if (data.peerIDs.includes(this.model.id)) {
          this.setBlockedClass();
        }
      });
    },

    onClickLoadingCancel() {
      if (window.history.state.back === null) {
        // there is no previous page, let's navigate to our home page
        this.$router.push({ path: `/${app.profile.id}`});
      } else {
        // go back to previous page
        this.$router.back();
      }
    },

    onClickLoadingRetry() {
      this.user(guid, state, ...args);
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

    clickTab(e) {
      const tab = $(e.target).closest('.js-tab').attr('data-tab');
      recordEvent('UserPage_Tab', { tab });
      this.setTabState(tab);
    },

    clickMore() {
      this.$moreableBtns.toggleClass('hide');
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
      this.isBlockedUser = isBlocked(this.model.id);
    },

    createFollowersTabView(opts = {}) {
      const collection = new Followers([], {
        peerID: this.model.id,
        type: 'followers',
      });

      this.listenTo(collection, 'sync', () => {
        this.followerCount = collection.length;
      });

      return this.createChild(this.tabViews.Follow, {
        ...opts,
        followType: 'followers',
        peerID: this.model.id,
        collection,
      });
    },

    createFollowingTabView(opts = {}) {
      const models = app.profile.id === this.model.id ? app.ownFollowing.models : [];
      const collection = new Followers(models, {
        peerID: this.model.id,
        type: 'following',
        fetchCollection: app.profile.id !== this.model.id,
      });

      this.listenTo(collection, 'sync', () => {
        this.followingCount = collection.length;
      });

      return this.createChild(this.tabViews.Follow, {
        ...opts,
        followType: 'following',
        peerID: this.model.id,
        collection,
      });
    },

    createStoreTabView(opts = {}) {
      this.listings = new Listings([], { guid: this.model.id });

      let listingsCount = this.model.get('listingCount');

      this.listings.on('update', () => {
        if (this.listings.length !== listingsCount) {
          listingsCount = this.listings.length;
          $('.js-listingsCount').html(abbrNum(listingsCount));
        }
      });

      return this.createChild(this.tabViews.Store, {
        ...opts,
        initialFetch: Store.fetchListings(this.listings),
        collection: this.listings,
        model: this.model,
      });
    },

    setTabState(state, options = {}) {
      if (!state) {
        throw new Error('Please provide a state.');
      }

      this.state = state;
      this.selectTab(state, options);
    },

    selectTab(targ, options = {}) {
      const opts = {
        addTabToHistory: true,
        ...options,
      };

      if (!this.tabViews[capitalize(targ)] && targ !== 'following' && targ !== 'followers') {
        throw new Error(`${targ} is not a valid tab.`);
      }

      let tabView = this.tabViewCache[targ];
      const tabOptions = {
        ownPage: this.ownPage,
        model: this.model,
        ...opts,
      };

      // delete any opts that the tab view(s) wouldn't need
      delete tabOptions.addTabToHistory;

      if (!this.currentTabView || this.currentTabView !== tabView) {
        const tabName = app.polyglot.t(`userPage.tabTitles.${targ}`);
        this.$tabTitle.text(tabName);

        if (opts.addTabToHistory) {
          const listingBaseUrl = this.model.get('handle') ? `@${this.model.get('handle')}` : this.model.id;

          // add tab to history
          app.router.navigateUser(`${listingBaseUrl}/${targ.toLowerCase()}`, this.model.id);
        }

        $('.js-tab').removeClass('clrT active');
        $(`.js-tab[data-tab="${targ}"]`).addClass('clrT active');

        if (this.currentTabView) this.currentTabView.$el.detach();

        if (!tabView) {
          if (this[`create${capitalize(targ)}TabView`]) {
            tabView = this[`create${capitalize(targ)}TabView`](tabOptions);
          } else {
            tabView = this.createChild(this.tabViews[capitalize(targ)], tabOptions);
          }

          this.tabViewCache[targ] = tabView;
          tabView.render();
        }

        this.$tabContent.append(tabView.$el);
        this.currentTabView = tabView;
      }
    },

    render() {
      this.$tabContent = $('.js-userPage-tabContent');
      this.$tabTitle = $('.js-tabTitle');
      this.$moreableBtns = $('.js-moreableBtn');

      this.tabViewCache = {}; // clear for re-renders
      this.setTabState(this.state, {
        addTabToHistory: false,
        listing: this.options.listing,
      });

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
