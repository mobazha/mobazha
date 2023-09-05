<template>
  <div class="convoMessage">
    <div v-if="!ob.outgoing">
      <div class="gutterH flex rowLg">
        <a :href="`#${ob.peerID}`" :class="`avatar disc clrBr2 clrSh1 flexNoShrink ${!ob.showAvatar ? 'invisible' : ''}`"
          :style="ob.showAvatar ? ob.getAvatarBgImage(ob.avatarHashes) : ''"></a>
        <div class="posR flexExpand">
          <div :class="`contentBox msgContentBox clrBr clrP clrSh2 ${ob.showTimestampLine ? 'rowSm' : ''}`">
            <span class="tx5">{{ ob.processedMessage }}</span>
          </div>
          <div v-if="ob.showTimestampLine">
            <div>
              <span class="clrT2 tx6">{{ timeLine }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-else>
      <div class="flexHRight gutterH rowLg">
        <div class="posR flexExpand">
          <div
            :class="`contentBox msgContentBox clrBr clrP clrSh2 <% if (ob.showAsRead) print('read') %> ${ob.showTimestampLine ? 'rowSm' : ''}`">
            <span class="tx5">{{ ob.processedMessage }}</span>
          </div>
          <div v-if="ob.showTimestampLine">
            <div>
              <span class="clrT2 tx6">{{ timeLine }}</span>
            </div>
          </div>
        </div>
        <a :href="`#${app.profile.id}`"
          :class="`avatar disc clrBr2 clrSh1 flexNoShrink ${!ob.showAvatar ? 'invisible' : ''}`"
          :style="ob.showAvatar ? ob.getAvatarBgImage(ob.avatarHashes) : ''"></a>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import moment from 'moment';
import twemoji from 'twemoji';
import { capitalize } from '../../../../backbone/utils/string';
import { setTimeagoInterval } from '../../../../backbone/utils';

export default {
  mixins: [],
  props: {
    cart: Object,
  },
  data () {
    return {
      showAvatar: true,
      showTimestampLine: true,
      showAsRead: false,
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
    timeLine () {
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
