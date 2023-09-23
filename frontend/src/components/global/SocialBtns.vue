<template>
  <div class="socialBtns">
    <div :class="ob.stripClasses">
      <a :class="`btn ${ob.btnClasses}`" @click="onClickMessage">{{ ob.polyT('userPage.message') }}</a>
      <ProcessingButton
        :className="`btn ${ob.btnClasses} ${ob.isFollowing ? 'processing' : ''}`"
        @click="onClickFollow"
        :btnText="ob.following ? ob.polyT('follow.unfollowBtn') : ob.polyT('follow.followBtn')"
      />
      <div class="js-blockBtnContainer">
        <BlockBtn :options="{ targetID: options.targetID }" />
      </div>
    </div>
  </div>
</template>

<script>
import app from '../../../backbone/app';
import { followedByYou, followUnfollow } from '../../../backbone/utils/follow';
import { recordEvent } from '../../../backbone/utils/metrics';

export default {
  props: {
    params: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      options: {},
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.params);
  },
  mounted() {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.options,
        ...this._state,
      };
    },
  },
  methods: {
    loadData(options = {}) {
      console.log('options: ', options)
      if (!options.targetID) throw new Error('You must provide a targetID');

      const opts = {
        ...options,
        initialState: {
          following: followedByYou(options.targetID),
          isFollowing: false,
          stripClasses: 'btnStrip clrSh3',
          btnClasses: 'clrP clrBr',
          ...(options.initialState || {}),
        },
      };

      this.setState(opts.initialState || {});
      this.options = opts;

      this.listenTo(app.ownFollowing, 'update', () => {
        this.setState({
          following: followedByYou(options.targetID),
        });
      });
    },

    className() {
      return 'socialBtns';
    },

    onClickMessage() {
      // activate the chat message
      app.chat.openConversation(this.options.targetID);
      recordEvent('Social_OpenChat');
    },

    onClickFollow() {
      const type = this.getState().following ? 'unfollow' : 'follow';
      this.setState({ isFollowing: true });
      this.folCall = followUnfollow(this.options.targetID, type).always(() => {
        if (this.isRemoved()) return;
        this.setState({ isFollowing: false });
      });
      if (type === 'follow') {
        recordEvent('Social_Follow');
      } else {
        recordEvent('Social_Unfollow');
      }
    },
  },
};
</script>
<style lang="scss" scoped></style>
