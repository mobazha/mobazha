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
          <div class="rowTn clamp2 notifMsg">{{ ob.notifText }}</div>
          <div class="clrT2 tx6">{{ ob.renderedTimeAgo }}</div>
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
      this.renderedTimeAgo = moment(this.model.get('timestamp')).fromNow();

      return {
        ...this.templateHelpers,
        ...this._state,
        ...this.model.toJSON(),
        notifText: this.getNotifDisplayData().text,
        ownGuid: app.profile.id,
        renderedTimeAgo: this.renderedTimeAgo,
        moment,
      };
    },

    isOrderNotif() {
      const notification = this.model.toJSON().notification;
      return !!notification.orderID || !!notification.purchaseOrderID || !!notification.disputeCaseId;
    }
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        initialState: {
          ...options.initialState || {},
        },
        ...options,
      };

      if (!options.model) {
        throw new Error('Please provide a model.');
      }

      this.baseInit(opts);
      this.options = opts;
      this.listenTo(this.model, 'change', () => this.render());

      this.timeAgoInterval = setTimeagoInterval(this.model.get('timestamp'), () => {
        const timeAgo = moment(this.model.get('timestamp')).fromNow();
        if (timeAgo !== this.renderedTimeAgo) this.render();
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

    remove(){
      this.timeAgoInterval.cancel();
      super.remove();
    },
  }
}
</script>
<style lang="scss" scoped></style>
