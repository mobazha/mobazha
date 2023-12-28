<template>
  <div class="notification" @click.stop.prevent="onClick">
    <div :class="`listItem notificationListItem js-notificationListItem clrBr ${!ob.read ? 'unread' : ''}`">
      <div class="unreadBorder clrBAtt1"></div>
      <div class="flexVCent gutterHSm pad">
        <div v-if="ob.notification.type === 'payment'" class="thumbCol flexNoShrink>">
          {{ ob.crypto.cryptoIcon({ code: ob.notification.coinType || 'UNKNOWN_CODE' }) }}
        </div>
        <div v-else :class="`thumbCol disc clrBr2 clrSh1 flexNoShrink ${isOrderNotif ? 'listingImage' : ''}`"
          :style="ob[isOrderNotif ? 'getListingBgImage' : 'getAvatarBgImage'](ob.notification.thumbnail || ob.notification.avatarHashes || {})">
        </div>
        <div class="flexExpand">
          <div class="rowTn clamp2 notifMsg" v-html="ob.notifText"></div>
          <div class="clrT2 tx6">{{ renderedTimeAgo }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import moment from 'moment';
import app from '../../../backbone/app';
import { getNotifDisplayData } from '../../../backbone/collections/Notifications';
import { setTimeagoInterval } from '../../../backbone/utils';
import { recordEvent } from '../../../backbone/utils/metrics';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      renderedTimeAgo: '',
    };
  },
  created() {
    this.initEventChain();

    this.loadData();
  },
  mounted() {
  },
  unmounted () {
    this.timeAgoInterval.cancel();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
        ...this.model.toJSON(),
        notifText: this.getNotifDisplayData().text,
        ownGuid: app.profile.id,
        moment,
      };
    },

    isOrderNotif() {
      const notification = this.model.toJSON().notification;
      return !!notification.orderID || !!notification.purchaseOrderID || !!notification.disputeCaseId;
    },
  },
  methods: {
    loadData() {
      if (!this.model) {
        throw new Error('Please provide a model.');
      }

      this.renderedTimeAgo = moment(this.model.get('timestamp')).fromNow();
      this.timeAgoInterval = setTimeagoInterval(this.model.get('timestamp'), () => {
        this.renderedTimeAgo = moment(this.model.get('timestamp')).fromNow();
      });
    },

    onClick() {
      recordEvent('Notifications_NotificationClick', {
        type: this.model.get('type'),
        read: this.model.get('read'),
      });
      const route = this.getNotifDisplayData().route;
      if (route) {
        location.hash = route;
        this.$emit('navigate');
      }
    },

    getNotifDisplayData() {
      return getNotifDisplayData(this.model.toJSON().notification);
    },
  }
}
</script>
<style lang="scss" scoped></style>
