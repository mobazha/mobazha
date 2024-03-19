<template>
  <div :class="`notificationsList navList listBox ${!collection.length ? 'noNotifications' : ''}`">
    <div class="js-notifsContainer reverseorder">
      <Notification
        v-for="(notif, index) in collection"
        :key="notif.id"
        ref="notifViews"
        @navigate="$emit('notifNavigate')"
        :bb="
          () => {
            return {
              model: notif,
            };
          }
        "
      />
    </div>
    <div class="js-fetcherContainer fetcherContainer">
      <ListFetcher :fetchState="fetchState" @retry-click="onRetryClick" />
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import { getSocket } from '../../../backbone/utils/serverConnect';
import { getCachedProfiles } from '../../../backbone/models/profile/Profile';
import Notifications from '../../../backbone/collections/Notifications';
import ListFetcher from './ListFetcher.vue';
import Notification from './Notification.vue';

export default {
  components: {
    ListFetcher,
    Notification,
  },
  props: {
    options: {
      type: Object,
      default: {
        filter: '',
        scrollContainer: undefined,
      },
    },
  },
  data() {
    return {
      notifsPerFetch: 4, // 20

      _collection: new Notifications(),
      collectionKey: 0,

      fetchState: {
        isFetching: false,
        fetchFailed: false,
        fetchError: '',
      },
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
    this.fetchState = {
      isFetching: this.notifFetch && this.notifFetch.state() === 'pending',
      fetchFailed: this.notifFetch && this.notifFetch.state() === 'rejected',
      fetchError: (this.notifFetch && this.notifFetch.responseJSON && this.notifFetch.responseJSON.reason) || '',
    };
  },
  unmounted() {
    if (this.notifFetch) this.notifFetch.abort();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        notifications: this.collection.toJSON(),
      };
    },
    collection() {
      let access = this.collectionKey;

      return this._collection;
    },
  },
  methods: {
    loadData() {
      // This count represents the total number of notifications that this list
      // is to show. It's used to know when all pages have been loaded. It's determined
      // based off of the returned total from the fetch of the first page + any new
      // notifications that come over the socket.
      this.totalNotifs = 0;

      this.listenTo(this._collection, 'update', (cl, updateOpts) => {
        this.collectionKey += 1;

        if (updateOpts.changes.added.length) {
          updateOpts.changes.added.forEach((notif) => {
            const innerNotif = notif.get('notification');
            const types = ['follow', 'moderatorAdd', 'moderatorRemove'];

            if (types.indexOf(innerNotif.type) > -1) {
              getCachedProfiles([innerNotif.peerID])[0].done((profile) => {
                notif.set('notification', {
                  ...innerNotif,
                  handle: profile.get('handle') || '',
                  avatarHashes: (profile.get('avatarHashes') && profile.get('avatarHashes').toJSON()) || {},
                });
              });
            }
          });

          this.fetchState.noResults = false;
        }
      });

      const serverSocket = getSocket();

      if (serverSocket) {
        serverSocket.on('message', (e) => {
          if (e.jsonData.notification && e.jsonData.notification.type !== 'unfollow') {
            const { type } = e.jsonData.notification;
            const filters = (this.options.filter || '')
              .split(',')
              .filter((filter) => filter.trim().length)
              .map((filter) => filter.trim());

            if (!filters.length || filters.indexOf(type) > -1) {
              this.totalNotifs += 1;
              this._collection.add({
                id: e.jsonData.notification.notificationID,
                read: false,
                timestamp: new Date().toISOString(),
                notification: {
                  ..._.omit(e.jsonData.notification, 'notificationID'),
                },
              });
            }
          }
        });
      }

      this.fetchNotifications();
    },
    onScroll() {
      const isFetching = this.notifFetch && this.notifFetch.state() === 'pending';

      const fetchFailed = this.notifFetch && this.notifFetch.state() === 'rejected';

      const allLoaded = this.collection.length >= this.totalNotifs;

      if (this.collection.length && !allLoaded && !isFetching && !fetchFailed && this.$refs.notifViews) {
        // fetch next batch of notifications
        const lastNotif = this.$refs.notifViews[this.$refs.notifViews.length - 1];

        if (this.isNotifScrolledIntoView(lastNotif.$el)) {
          this.fetchNotifications();
        }
      }
    },

    /*
     * isScrolledIntoView from util/dom.js is not accurately returning a result for
     * a notification because the notifications menu markup is inside the very narrow
     * pageNav bar. Since the notification is outside the pageNav's "viewport", it thinks
     * nothing within the notif menu is ever in view. It's a unique enough case that we'll
     * create a custom function here.
     */
    isNotifScrolledIntoView(notifEl) {
      const notifRect = notifEl.getBoundingClientRect();
      const scrollRect = $('.js-tabNotifContainer')[0].getBoundingClientRect();
      return -Math.floor(notifRect.top) <= scrollRect.top + $('.js-tabNotifContainer')[0].clientHeight;
    },

    fetchNotifications() {
      if (this.notifFetch) this.notifFetch.abort();

      const fetchParams = {
        limit: this.notifsPerFetch,
      };

      if (this._collection.length) {
        fetchParams.offsetID = this._collection.at(0).id;
      }

      if (this.options.filter) {
        fetchParams.filter = this.options.filter;
      }

      this.notifFetch = this._collection.fetch({
        data: fetchParams,
        remove: false,
      });

      this.fetchState = {
        isFetching: true,
        fetchFailed: false,
        noResults: false,
        fetchError: '',
      };

      this.notifFetch
        .done((data, txtStatus, xhr) => {
          if (xhr.statusText === 'abort') return;

          this.collectionKey += 1;

          const state = {
            isFetching: false,
            fetchFailed: false,
            noResults: false,
            fetchError: '',
          };

          if (!fetchParams.offsetID) {
            this.totalNotifs += data.total;

            if (!data.notifications.length) {
              state.noResults = true;
            }
          }

          this.fetchState = state;
        })
        .fail((xhr) => {
          this.fetchState = {
            isFetching: false,
            fetchFailed: true,
            fetchError: (xhr.responseJSON && xhr.responseJSON.reason) || '',
          };
        });
    },

    onRetryClick() {
      // Timeout is needed because otherwise when the listFetcher state changes and
      // the retry button is no longer in the dom, the doc click handler in pagenav
      // closes the menu. The notifContainer stops bubbling, so it shouldn't make it
      // to the doc handler, but something gets wonky if it's ripped out of the dom.
      setTimeout(() => {
        this.fetchNotifications();
      });
    },
  },
};
</script>
<style lang="scss" scoped>
.reverseorder {
  display: flex;
  flex-direction: column-reverse;
}
</style>
