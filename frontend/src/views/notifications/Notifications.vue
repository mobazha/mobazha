<template>
  <section class="notifications clrBr border clrP clrSh1">
    <h1>{{ ob.polyT('notifications.title') }}</h1>
    <div class="flex tabs">
      <template v-for="tab in ['all', 'orders', 'followers']">
        <a :class="`js-tab clrBr clrT2 ${tab === activeTab ? 'active' : ''}`" @click="onClickTab(tab)">{{ ob.polyT(`notifications.tab${capitalize(tab)}`) }}</a>
      </template>
    </div>
    <div v-infinite-scroll="onScroll" ref="tabContainer" class="js-tabNotifContainer tabContainer scrollBox clrBr">
      <NotificationsList :key="activeTab" ref="notifLists" :options="{ filter, scrollContainer }" @notifNavigate="$emit('notifNavigate', { list })" />
    </div>
  </section>
</template>

<script>
import $ from 'jquery';
import app from '../../../backbone/app';
import { myPost } from '../../api/api';
import { capitalize } from '../../../backbone/utils/string';
import { recordEvent } from '../../../backbone/utils/metrics';

import NotificationsList from './NotificationsList.vue';

export default {
  components: {
    NotificationsList,
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
      activeTab: 'all',

      list: 'all',
    };
  },
  created() {
    this.initEventChain();
  },
  mounted() {},
  computed: {
    filter() {
      switch (this.activeTab) {
        case 'orders':
          this.list = 'order';
          return (
            'order,orderDeclined,cancel,refund,fulfillment,orderComplete,disputeOpen,' +
            'disputeUpdate,disputeClose,disputeAccepted,vendorDisputeTimeout,buyerDisputeTimeout' +
            'buyerDisputeExpiry,moderatorDisputeExpiry'
          );
        case 'followers':
          this.list = 'follow';
          return 'follow';
        default:
          this.list = 'all';
          return '';
      }
    },

    scrollContainer() {
      return $('.js-tabNotifContainer');
    },
  },
  methods: {
    onScroll() {
      this.$refs.notifLists.onScroll();
    },
    capitalize,

    loadData() {
      this.activeTab = this.options.tab;
    },

    onClickTab(tab) {
      recordEvent('Notifications_Tab', { tab });
      // Timeout needed so event can bubble to a page nav handler before the view is re-rendered
      // and the target element is ripped out of the dom.
      setTimeout(() => {
        this.activeTab = tab;
      });
    },

    /**
     * If there are any loaded notifications, this method will kick off a server
     * call that will mark all notifications (seen and unseen) as read. If there
     * are no loaded notifications (possibly because a initial page is being fetched),
     * it will return false and not kick off any server call.
     * @return {boolean|object} False if no notifications have been loaded, otherwise
     *   the xhr of the call to the server
     */
    markNotifsAsRead() {
      // Going to optimistically mark all as read and switch back if the call fails.
      const notifs = [];

      if (this.$refs.notifLists) {
        this.$refs.notifLists.collection.forEach((notif) => {
          notif.set('read', true);
          notifs.push(notif);
        });
      }

      return myPost(app.getServerUrl('ob/marknotificationsasread')).fail(() => notifs.forEach((notif) => notif.set('read', false)));
    },

    /**
     * Will set the tab to 'All' and set the scroll position to the top - useful
     * when hiding the menu so that it resets to a standard initial position. It will
     * leave the collections intact, so the user won't need to fetch notifications
     * already fetched.
     */
    reset() {
      this.activeTab = 'all';
      this.scrollContainer.scrollTop = 0;
    },
  },
};
</script>
<style lang="scss" scoped></style>
