<template>
  <div class="convoMessage">
    <template v-if="!ob.outgoing">
      <div class="gutterH flex rowLg">
        <a :href="`#${ob.peerID}`" :class="`avatar disc clrBr2 clrSh1 flexNoShrink ${!ob.showAvatar ? 'invisible' : ''}`"
          :style="ob.showAvatar ? ob.getAvatarBgImage(ob.avatarHashes) : ''"></a>
        <div class="posR flexExpand">
          <div :class="`contentBox msgContentBox clrBr clrP clrSh2 ${ob.showTimestampLine ? 'rowSm' : ''}`">
            <span class="tx5">{{ ob.processedMessage }}</span>
          </div>
          <template v-if="ob.showTimestampLine">
            <div>
              <span class="clrT2 tx6">{{ timeLine }}</span>
            </div>
          </template>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="flexHRight gutterH rowLg">
        <div class="posR flexExpand">
          <div
            :class="`contentBox msgContentBox clrBr clrP clrSh2 ${ob.showAsRead ? 'read' : ''} ${ob.showTimestampLine ? 'rowSm' : ''}`">
            <span class="tx5">{{ ob.processedMessage }}</span>
          </div>
          <template v-if="ob.showTimestampLine">
            <div>
              <span class="clrT2 tx6">{{ timeLine }}</span>
            </div>
          </template>
        </div>
        <a :href="`#${ob.ownGuid}`"
          :class="`avatar disc clrBr2 clrSh1 flexNoShrink ${!ob.showAvatar ? 'invisible' : ''}`"
          :style="ob.showAvatar ? ob.getAvatarBgImage(ob.avatarHashes) : ''"></a>
      </div>
    </template>

  </div>
</template>

<script>
import $ from 'jquery';
import moment from 'moment';
import twemoji from 'twemoji';
import { capitalize } from '../../../../backbone/utils/string';
import { setTimeagoInterval } from '../../../../backbone/utils';
import app from '../../../../backbone/app';

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
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._model,
        ...this._state,
        moment,
        message,
        capitalize,
        ownGuid: app.profile.id,
      };
    },

    timeLine () {
      const ob = this.ob;
      return ob.polyT('orderDetail.discussionTab.timeLineAndRole', {
        timeFromNow: moment(ob.timestamp).fromNow(),
        role: ob.polyT(`orderDetail.discussionTab.role${capitalize(ob.role)}`)
      });
    },
  },
  methods: {
    loadData (options = {}) {
      if (!options.model) {
        throw new Error('Please provide a model.');
      }

      this.baseInit({
        ...options,
        initialState: {
          showAvatar: true,
          showTimestampLine: true,
          showAsRead: false,
          ...options.initialState,
        },
      });

      this.listenTo(this.model, 'change', () => this.render());
      this.timeAgoInterval = setTimeagoInterval(this.model.get('timestamp'), () => {
        const timeAgo = moment(this.model.get('timestamp')).fromNow();
        if (timeAgo !== this.renderedTimeAgo) this.render();
      });
    },

    remove () {
      this.timeAgoInterval.cancel();
      super.remove();
    },

    render () {
      let message = this.model.get('message');

      // Give any links the emphasis color.
      const $msgHtml = $(`<div>${message}</div>`);

      $msgHtml.find('a').addClass('clrTEm');

      // Convert any unicode emoji characters to images via Twemoji
      message = twemoji.parse($msgHtml.html(), icon => (`../imgs/emojis/72X72/${icon}.png`));

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
