<template>
  <div class="socialBtns">
    <div :class="ob.stripClasses">
      <a :class="`btn ${ob.btnClasses}`" @click="onClickMessage">{{ ob.polyT('userPage.message') }}</a>
      <ProcessingButton
        :className="`btn ${ob.btnClasses} ${ob.isFollowing ? 'processing' : ''}`"
        @click="onClickFollow"
        :btnText="ob.following ? ob.polyT('follow.unfollowBtn') : ob.polyT('follow.followBtn')" />
      <div class="js-blockBtnContainer"></div>
    </div>

  </div>
</template>

<script>
import app from '../../../backbone/app';
import loadTemplate from '../../../backbone/utils/loadTemplate';
import { followedByYou, followUnfollow } from '../../../backbone/utils/follow';
import BlockBtn from './BlockBtn';
import { recordEvent } from '../../../backbone/utils/metrics';

import BaseVw from '../baseVw';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
    params () {
      return {
        ...this.options,
        ...state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.targetID) throw new Error('You must provide a targetID');

      const opts = {
        ...options,
        initialState: {
          following: followedByYou(options.targetID),
          isFollowing: false,
          stripClasses: 'btnStrip clrSh3',
          btnClasses: 'clrP clrBr',
          ...options.initialState || {},
        },
      };

      super(opts);
      this.options = opts;

      this.listenTo(app.ownFollowing, 'update', () => {
        this.setState({
          following: followedByYou(options.targetID),
        });
      });
    },

    className () {
      return 'socialBtns';
    },

    events () {
      return {
        'click .js-followUnfollowBtn': 'onClickFollow',
        'click .js-messageBtn': 'onClickMessage',
      };
    },

    onClickMessage () {
      // activate the chat message
      app.chat.openConversation(this.options.targetID);
      recordEvent('Social_OpenChat');
    },

    onClickFollow () {
      const type = this.getState().following ? 'unfollow' : 'follow';
      this.setState({ isFollowing: true });
      this.folCall = followUnfollow(this.options.targetID, type)
        .always(() => {
          if (this.isRemoved()) return;
          this.setState({ isFollowing: false });
        });
      if (type === 'follow') {
        recordEvent('Social_Follow');
      } else {
        recordEvent('Social_Unfollow');
      }
    },

    render () {
      super.render();
      const state = this.getState();
      loadTemplate('components/socialBtns.html', (t) => {
        this.$el.html(t({
          ...this.options,
          ...state,
        }));
      });

      this.getCachedEl('.js-blockBtnContainer')
        .html(
          new BlockBtn({ targetId: this.options.targetID })
            .render()
            .el
        );

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
