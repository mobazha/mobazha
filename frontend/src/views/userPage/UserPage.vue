<template>
  <div :class="`userPage clrS ${isBlockedUser ? 'isBlocked' : ''}`">
    <nav id="pageTabBar" class="barLg clrP clrBr">
      <div class="flexVCent pageTabs">
        <div class="js-miniProfileContainer"></div>
        <div class="flexExpand">
          <div class="flexHRight flexVCent gutterH clrT2">
            <a class="btn tab clrBr js-tab" @click="clickTab" data-tab="home">{{ ob.polyT('userPage.mainNav.home') }}</a>
            <!-- // the store tab is only visible to the user if they have vendor set to false -->
            <a v-if="ob.vendor || ob.ownPage" class="btn tab clrBr js-tab" @click="clickTab" data-tab="store">
              {{ ob.polyT('userPage.mainNav.store') }}<span class="clrTEmph1 margLSm js-listingsCount">{{ ob.stats.listingCount }}</span></a>
            <a class="btn tab clrBr js-tab" @click="clickTab" data-tab="following">
              {{ ob.polyT('userPage.mainNav.following') }}<span class="clrTEmph1 margLSm js-followingCount">{{ ob.followingCount }}</span></a>
            <a class="btn tab clrBr js-tab" @click="clickTab" data-tab="followers">
              {{ ob.polyT('userPage.mainNav.followers') }}<span class="clrTEmph1 margLSm js-followerCount">{{ ob.followerCount }}</span></a>
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
          <div v-if="ob.ownPage">
            <div class="btnStrip floR clrSh2">
              <a class="btn clrP clrBr" @click="clickCustomize">{{ ob.polyT('userPage.customize') }}</a>
              <a class="btn clrP clrBr" @click="clickCreateListing">{{ ob.polyT('userPage.createListing') }}</a>
              <!--
          <a class="btn clrP clrBr hide js-moreableBtn">{{ ob.polyT('userPage.block') }}</a>
          <a class="iconBtn clrP clrBr " @click="clickMore" ><i class="ion-android-more-vertical"></i> </a>
        -->
            </div>
          </div>

          <div v-else>
            <div class="js-socialBtns"></div>
            <SocialBtns :targetID="model.id" />
          </div>
          <div v-if="ob.showStoreWelcomeCallout">
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
          </div>
        </div>
      </div>
      <div class="tabContent js-tabContent">
        <!-- insert the tab subview here -->
      </div>
    </div>

    <Teleport to="#js-vueModal">
      <Loading v-if="showUserLoading" ref="userLoadingModal" :options="{
          initialState: {
            contentText: loadingContextText,
            isProcessing: isLoadingUser,
          },
        }"
        @clickCancel="onClickLoadingCancel"
        @clickRetry="onClickLoadingRetry"/>
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
import { isBlocked, events as blockEvents } from '../../../backbone/utils/block';
import { getCurrentConnection } from '../../../backbone/utils/serverConnect';
import Profile from '../../../backbone/models/profile/Profile';
import Listing from '../../../backbone/models/listing/Listing';
import Listings from '../../../backbone/collections/Listings';
import Followers from '../../../backbone/collections/Followers';
import MiniProfile from '../../../backbone/views/MiniProfile.js';
import Home from '../../../backbone/views/userPage/Home';
import Store from '../../../backbone/views/userPage/Store';
import Follow from '../../../backbone/views/userPage/Follow';
import Reputation from '../../../backbone/views/userPage/Reputation';

import Loading from './Loading.vue'

export default {
  components: {
    Loading,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      handle: '',
      guild: '',
      state: '',
      slug: '',

      profile: {},
      profileFetch: undefined,
      listing: {},
      listingFetch: undefined,

      model: {},
      isBlockedUser: false,

      loadingContextText: '',
      isLoadingUser: true,
      showUserLoading: true,

      loadingUserFailed: false,
    };
  },
  created() {
    this.initEventChain();

    this.init();

    console.log('In userPage')
  },
  mounted() {
    this.render();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        ownPage: this.ownPage,
        showStoreWelcomeCallout: this.showStoreWelcomeCallout,
        followingCount: abbrNum(this.followingCount),
        followerCount: abbrNum(this.followerCount),
      };
    },
    headerHash() {
      const ob = this.ob;
      return ob.headerHashes ? (ob.isHiRez() ? ob.headerHashes.large : ob.headerHashes.medium) : '';
    },
  },
  watch: {
    $route() {
      if (!this.loadingUserFailed) {
        // The app has been routed to a new route, let's
        // clean up by aborting all fetches
        if (this.profileFetch?.abort) this.profileFetch.abort();
        if (this.listingFetch) this.listingFetch.abort();
      }
    }
  },
  methods: {
    init() {
      // Hack to pass the handle into this function, which should really only
      // happen when called from userViaHandle(). If a handle is being passed in,
      // it will be passed in as { handle: 'charlie' } as the first element of the
      // ...args argument.
      let handle;

      let {guid, state, slug} = this.$route.params;
      this.guid = guid;
      this.state = state;
      this.slug = slug;

      const pageState = state || 'store';

      let userPageFetchError = '';

      startAjaxEvent('UserPageLoad');

      if (guid === app.profile.id) {
        // don't fetch our own profile, since we have it already
        this.profileFetch = $.Deferred().resolve();
        this.profile = app.profile;
      } else {
        this.profile = new Profile({ peerID: guid });
        this.profileFetch = this.profile.fetch();
      }

      if (state === 'store') {
        if (slug) {
          this.listing = new Listing({ slug, }, { guid });

          this.listingFetch = this.listing.fetch();
        }
      }

      app.loadingModal.close();

      this.loadingUserFailed = false;

      this.loadingContextText = app.polyglot.t('userPage.loading.loadingText', { name: `<b>${handle || `${guid.slice(0, 8)}…`}</b>`, });
      this.isLoadingUser = true;

      $.whenAll(this.profileFetch, this.listingFetch).done(() => {
        this.showUserLoading = false;
        // handle = profile.get('handle');
        // this.cacheGuidHandle(guid, handle);

        this.loadData({
          model: this.profile,
          state: pageState,
          listing: this.listing,
        })
      }).fail((...failArgs) => {
        const jqXhr = failArgs[0];
        const reason = (jqXhr && jqXhr.responseJSON && jqXhr.responseJSON.reason)
          || (jqXhr && jqXhr.responseText) || '';

        if (jqXhr === this.profileFetch && this.profileFetch.statusText === 'abort') return;
        if (jqXhr === this.listingFetch && this.listingFetch.statusText === 'abort') return;

        if (this.profileFetch.state() === 'rejected') {
          userPageFetchError = 'User Not Found';
        } else if (this.listingFetch.state() === 'rejected') {
          userPageFetchError = 'Listing Not Found';
        }

        userPageFetchError = userPageFetchError
          ? `${userPageFetchError} - ${reason || 'unknown'}`
          : reason || 'unknown';

        let contentText = app.polyglot.t('userPage.loading.failTextStore', {
          store: `<b>${handle || `${guid.slice(0, 8)}…`}</b>`,
        });

        if (this.profileFetch.state() === 'resolved' && this.listingFetch.state() === 'rejected') {
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
      }).always(() => {
        this.loadingUserFailed = true;

        const dismissedCallout = getCurrentConnection()
          && getCurrentConnection().server.get('dismissedDiscoverCallout');
        endAjaxEvent('UserPageLoad', {
          ownPage: guid === app.profile.id,
          tab: pageState,
          dismissedCallout,
          listing: !!this.listingFetch,
          errors: userPageFetchError || 'none',
        });
      });
    },
    loadData(options = {}) {
      this.baseInit(options);

      if (!options.model) {
        throw new Error('Please provide a user model.');
      }

      this.setBlockedClass();

      this.ownPage = this.model.id === app.profile.id;
      this.state = options.state || 'store';
      this.tabViewCache = {};
      this.tabViews = { Home, Store, Follow, Reputation };

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

      this.listenTo(this.model.get('headerHashes'), 'change', () => this.updateHeader());

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

        if (this.followingCount === 0 && !this.ownPage) this.setFollowingCount(1);
      });

      this.listenTo(blockEvents, 'blocked unblocked', (data) => {
        if (data.peerIDs.includes(this.model.id)) {
          this.setBlockedClass();
        }
      });
    },

    onClickLoadingCancel() {

    },

    onClickLoadingRetry() {
      this.init();
    },

    onOwnFollowingAdd(md) {
      if (this.ownPage) {
        this.setFollowingCount(this.followingCount + 1);
      } else if (md.id === this.model.id) {
        this.setFollowerCount(this.followerCount + 1);
      }
    },

    onOwnFollowingRemove(md) {
      if (this.ownPage) {
        this.setFollowingCount(this.followingCount - 1);
      } else if (md.id === this.model.id) {
        this.setFollowerCount(this.followerCount - 1);
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
        this.getCachedEl('.js-storeWelcomeCallout').remove();
      }
    },

    clickRating() {
      recordEvent('UserPage_ClickReputation');
      this.setTabState('reputation');
    },

    setFollowingCount(count) {
      if (typeof count !== 'number') {
        throw new Error('Please provide a numeric count.');
      }

      if (count !== this.followingCount) {
        this.followingCount = count;
        this.getCachedEl('.js-followingCount').text(abbrNum(count));
      }
    },
    setFollowerCount(count) {
      if (typeof count !== 'number') {
        throw new Error('Please provide a numeric count.');
      }

      if (count !== this.followerCount) {
        this.followerCount = count;
        this.getCachedEl('.js-followerCount').text(abbrNum(count));
      }
    },

    setBlockedClass() {
      this.isBlockedUser = isBlocked(this.model.id);
    },

    updateHeader() {
      const headerHashes = this.model.get('headerHashes').toJSON();
      const headerHash = isHiRez() ? headerHashes.large : headerHashes.medium;

      if (headerHash) {
        $('.js-header').attr(
          'style',
          `background-image: url(${app.getServerUrl(`ob/image/${headerHash}`)}),
        url('../imgs/defaultHeader.png')`
        );
      }
    },

    createFollowersTabView(opts = {}) {
      const collection = new Followers([], {
        peerID: this.model.id,
        type: 'followers',
      });

      this.listenTo(collection, 'sync', () => {
        this.setFollowerCount(collection.length);
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
        this.setFollowingCount(collection.length);
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
    remove() {
      if (this.followingFetch) this.followingFetch.abort();
    },

    render() {
      this.$tabContent = $('.js-tabContent');
      this.$tabTitle = $('.js-tabTitle');
      this.$moreableBtns = $('.js-moreableBtn');

      if (this.miniProfile) this.miniProfile.remove();
      this.miniProfile = this.createChild(MiniProfile, {
        model: this.model,
        fetchFollowsYou: false,
        onClickRating: () => this.setTabState('reputation'),
        initialState: {
          followsYou: this.followsYou,
        },
      });
      this.listenTo(this.miniProfile, 'clickRating', this.clickRating);
      $('.js-miniProfileContainer').html(this.miniProfile.render().el);

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
