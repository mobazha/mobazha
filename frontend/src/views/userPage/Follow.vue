<template>
  <div :class="`userPageFollow ${noResults ? 'noResults': ''}`">
    <div ref="userCardsContainer" :key="renderedClKey" class="js-userCardsContainer userCardsContainer flexRow">
      <template v-for="user in renderedCl" :key="user.id">
        <UserCard :options="{ guid: user.id }"/>
      </template>
    </div>
    <div class="js-followLoadingContainer followLoadingContainer">
      <FollowLoading ref="followLoading" :options="followLoadingOptions()" @retry-click="fetch()" />
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import '../../../backbone/lib/whenAll.jquery';
import app from '../../../backbone/app';
import { getContentFrame } from '../../../backbone/utils/selectors';
import { followsYou, followedByYou } from '../../../backbone/utils/follow';
import Followers from '../../../backbone/collections/Followers';
import FollowLoading from './FollowLoading.vue';

export default {
  components: {
    FollowLoading,
  },
  props: {
    options: {
      type: Object,
      default: {
        followType: 'followers',
        fetchCollection: true,
      },
    },
    bb: Function,
  },
  data() {
    return {
      usersPerPage: 12,
      noResults: false,

      followsYouFetch: undefined,
      followFetch: undefined,

      _state: {
        followType: 'followers',
        fetchCollection: true,
      },

      renderedCl: undefined,
      renderedClKey: 0,
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
    if (this.followFetch) this.followFetch.abort();
    if (this.followsYouFetch && this.followsYouFetch.abort) {
      this.followsYouFetch.abort();
    }
  },
  computed: {
    ownPage() {
      return this.options.peerID === app.profile.id;
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        followType: 'followers',
        fetchCollection: true,
        ...options,
      };

      this.baseInit(opts);

      if (!opts.peerID) {
        throw new Error('Please provide a peerID of the user who this list is for.');
      }

      const types = ['followers', 'following'];
      if (types.indexOf(opts.followType) === -1) {
        throw new Error(`followType must be one of ${types.join(', ')}`);
      }

      if (!this.collection) {
        throw new Error('Please provide a followers collection.');
      }

      this._origClParse = this.collection.parse;
      this.collection.parse = this.collectionParse.bind(this);
      this.followType = opts.followType;

      this.renderedCl = new Followers([], {
        peerID: opts.peerID,
        type: opts.followType,
      });
      this.listenTo(this.renderedCl, 'update', this.onCollectionUpdate);

      if (opts.fetchCollection) {
        this.fetch();
      } else {
        setTimeout(() => this.onCollectionFetched.call(this));
      }

      this.listenTo(app.ownFollowing, 'update', this.onOwnFollowingUpdate);
      this.throttledOnScroll = _.throttle(this.onScroll, 100).bind(this);
    },

    onScroll() {
      const cf = getContentFrame()[0];
      const scrollNearBot = cf.scrollTop >= cf.scrollHeight - cf.offsetHeight - 100;
      if (this.renderedCl.length < this.collection.length && this.$refs.followLoading && !this.$refs.followLoading.getState().isFetching && scrollNearBot) {
        // Some fake latency so a user doesn't just scroll down and load
        // hundreds of userCards which would kick off hundreds of profile
        // fetches.
        this.$refs.followLoading.setState({ isFetching: true });

        setTimeout(() => {
          const start = this.renderedCl.length;
          const end = start + this.usersPerPage;
          this.$refs.followLoading.setState({ isFetching: false });
          this.renderedCl.add(this.collection.models.slice(start, end));
        }, 500);
      }
    },

    onOwnFollowingUpdate(cl, opts) {
      if (opts.changes.added.length) {
        if (this.ownPage) {
          if (this.followType === 'following') this.collection.add(opts.changes.added);
        } else if (this.followType === 'followers') {
          const isUserNewlyFollowed = !!opts.changes.added.find((addMd) => addMd.id === this.model.id);

          if (isUserNewlyFollowed) {
            this.collection.add({ peerID: app.profile.id }, { at: 0 });
          }
        }
      }

      if (opts.changes.removed.length) {
        // If someone is looking at their own following list, we won't remove the user card of
        // users they've unfollowed. It's likely they're scrolling through their own followers
        // list and cleaning house and since the unfollow process takes a while, it could
        // be chaotic for the cards to just disappear at some later time. The follow button
        // state on the card will correctly reflect that the user is no longer followed.
        if (this.ownPage) return;

        if (this.followType === 'followers') {
          const isUserNewlyUnfollowed = !!opts.changes.removed.find((removedMd) => removedMd.id === this.model.id);
          if (isUserNewlyUnfollowed) {
            this.collection.remove(app.profile.id);
          }
        }
      }
    },

    onCollectionUpdate(cl, opts) {
      this.renderedClKey += 1;

      this.noResults = !cl.length;

      if (this.$refs.followLoading) {
        this.$refs.followLoading.setState({ noResults: !cl.length });
      }
    },

    onCollectionFetched() {
      const state = {
        isFetching: false,
        fetchFailed: false,
        noResults: false,
        fetchErrorMsg: '',
      };

      if (!this.collection.length) {
        state.noResults = true;
      }

      if (this.$refs.followLoading) this.$refs.followLoading.setState(state);
      this.renderedCl.add(this.collection.models.slice(0, this.usersPerPage));

      // If any additions / removal occur on the main collection (e.g. this view
      // is showing our own following list and we follow / unfollow someone; this view
      // is showing anothers followers list and our own node has followed / unfollowed
      // that user), we sync them over to the renderedCl.
      this.listenTo(this.collection, 'add', (md) => {
        this.renderedCl.add(md, { at: this.collection.models.indexOf(md) });
      });

      this.listenTo(this.collection, 'remove', (md) => {
        this.renderedCl.remove(md.id);
      });
    },

    /**
     * Other nodes followers lists are not up to date. Since we do have
     * up to date knowledge of our own following / followers data, if we
     * know some part of the other nodes followers lists are not accurate,
     * we'll adjust them.
     */
    collectionParse(response) {
      let users = [...response];

      if (!this.ownPage) {
        if (this.followType === 'followers') {
          const iFollow = followedByYou(this.model.id);
          if (iFollow) {
            if (users.indexOf(app.profile.id) === -1) {
              // I am not in their followers list but should be.
              users = [app.profile.id, ...users];
            }
          } else if (users.indexOf(app.profile.id) > -1) {
            // I am in their followers list when I shouldn't be.
            users.splice(users.indexOf(app.profile.id), 1);
          }
        } else if (typeof this.followsMe !== 'undefined') {
          if (this.followsMe) {
            if (users.indexOf(app.profile.id) === -1) {
              // I am not in their following list but should be.
              users = [app.profile.id, ...users];
            }
          } else if (users.indexOf(app.profile.id) > -1) {
            // I am in their following list when I shouldn't be.
            users.splice(users.indexOf(app.profile.id), 1);
          }
        }
      }

      return this._origClParse.call(this.collection, users);
    },

    followLoadingOptions() {
      let noResultsMsg;
      let fetchErrorTitle;

      if (this.followType === 'followers') {
        fetchErrorTitle = app.polyglot.t('userPage.followTab.followersFetchError');
        noResultsMsg = this.ownPage
          ? app.polyglot.t('userPage.followTab.noOwnFollowers')
          : app.polyglot.t('userPage.followTab.noFollowers', {
              name: this.model.get('handle') || `${this.model.id.slice(0, 8)}…`,
            });
      } else {
        fetchErrorTitle = app.polyglot.t('userPage.followTab.followingFetchError');
        noResultsMsg = this.ownPage
          ? app.polyglot.t('userPage.followTab.noOwnFollowing')
          : app.polyglot.t('userPage.followTab.noFollowing', {
              name: this.model.get('handle') || `${this.model.id.slice(0, 8)}…`,
            });
      }

      const isFetching = (this.followsYouFetch && this.followsYouFetch.state() === 'pending') || (this.followFetch && this.followFetch.state() === 'pending');

      return {
        initialState: {
          isFetching,
          fetchFailed: this.followFetch && this.followFetch.state() === 'rejected',
          fetchErrorTitle,
          fetchErrorMsg: (this.followFetch && this.followFetch.responseJSON && this.followFetch.responseJSON.reason) || '',
          noResultsMsg,
          noResults: !this.collection.length,
        },
      };
    },

    fetch() {
      if (this.$refs.followLoading) {
        this.$refs.followLoading.setState({
          isFetching: true,
          fetchFailed: false,
          fetchErrorMsg: '',
        });
      }

      const followsYouDeferred = $.Deferred();

      if (!this.ownPage && this.followType === 'following' && (!this.followsYouFetch || this.followsYouFetchFailed)) {
        this.followsYouFetch = followsYou(this.model.id).done((data) => {
          this.followsMe = data.followsMe;
          followsYouDeferred.resolve();
        });
      } else {
        followsYouDeferred.resolve();
      }

      followsYouDeferred.always(() => {
        this.followFetch = this.collection
          .fetch()
          .done(() => this.onCollectionFetched.call(this))
          .fail((xhr) => {
            if (xhr.statusText === 'abort') return;
            if (xhr === this.followFetch) {
              if (this.$refs.followLoading) {
                this.$refs.followLoading.setState({
                  isFetching: false,
                  fetchFailed: true,
                  fetchErrorMsg: (xhr.responseJSON && xhr.responseJSON.reason) || '',
                });
              }
            }
          });
      });
    },

    render() {
      getContentFrame().off('scroll', this.throttledOnScroll).on('scroll', this.throttledOnScroll);

      return this;
    },
  },
};
</script>
<style lang="scss" scoped></style>
