<template>
  <div :class="`miniprofile ${isBlockedUser ? 'isBlocked' : ''}`">
    <div class="flexVCent">
      <div class="flexNoShrink">
        <div class="userIconWrap">
          <a :href="`#${ob.peerID}`">
            <div class="userIcon disc clrBr2 clrSh1" :style="ob.getAvatarBgImage(ob.avatarHashes || {})"></div>
          </a>
          <div class="blockedAvatarOverlay disc clrBr2 clrSh1 clrP clrT"><i class="ion-eye-disabled center"></i></div>
        </div>
      </div>
      <div class="flexExpand">
        <h1 :class="`h2 txUnb txUnl clamp ${ob.name.length > 30 ? 'tx3' : ''}`"><a :href="`#${ob.peerID}`" class="clrT">{{ ob.name }}</a></h1>
        <div class="txt5b gutterHSm">
          <span class="clrT"
            v-html="`${ ob.parseEmojis('📍', '', { style: 'width: 10px' }) } ${ ob.location || ob.polyT('userPage.noLocation') }`"></span>
          <template v-if="ob.followsYou">
            <span v-html="`${ob.parseEmojis('👥', '', { style: 'width: 10px' })} ${ob.polyT('userPage.followsYou')}`"></span>
          </template>
          <a class="ratingStrip" @click="onClickRating"
            v-html="ob.formatRating(ob.stats.averageRating, ob.stats.ratingCount)"></a>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import app from '../../backbone/app';
import { followsYou } from '../../backbone/utils/follow';
import { isBlocked, events as blockEvents } from '../../backbone/utils/block';

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
      isBlockedUser: false,
      overwriteClickRating: false,

      _state: {
        followsYou: false,
      }
    };
  },
  created () {
    this.initEventChain();

    this.loadData();
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
        ...this.model.toJSON(),
      };
    }
  },
  watch: {
  },
  methods: {
    loadData () {
      if (!this.model) {
        throw new Error('Please provide a profile model.');
      }

      if (this.model.id !== app.profile.id) {
        followsYou(this.model.id).done((data) => {
          this.setState({ followsYou: data.followsMe });
        });

        this.listenTo(app.ownFollowers, 'add', (md) => {
          if (md.id === app.profile.id) {
            this.setState({ followsYou: true });
          }
        });

        this.listenTo(app.ownFollowers, 'remove', (md) => {
          if (md.id === app.profile.id) {
            this.setState({ followsYou: false });
          }
        });
      }

      this.setBlockedClass();
      this.listenTo(blockEvents, 'blocked unblocked', (data) => {
        if (data.peerIDs.includes(this.model.id)) {
          this.setBlockedClass();
        }
      });
    },

    onClickRating () {
      if (this.overwriteClickRating) {
        this.$emit('clickRating')
      } else {
        app.router.navigate(`ob://${this.model.id}/reputation`, { trigger: true });
      }
    },

    setBlockedClass () {
      this.isBlockedUser = isBlocked(this.model.id);
    },
  }
}
</script>
<style lang="scss" scoped></style>
