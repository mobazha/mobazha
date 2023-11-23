<template>
  <div>
    <div class="flexVCent gutterHSm rowTn">
      <template v-if="ob.showAvatar">
        <div class="avatar clrBr2 clrSh1 disc" :style="ob.getAvatarBgImage(ob.avatarHashes || {})"></div>
      </template>
      <div>{{ ob.name }}
        <a class="clrTEm" :href="`#${ob.peerID}`">{{ ob.handle && `@${ob.handle}` || `${ob.peerID.slice(0,
          ob.maxPeerIDLength)}…` }}</a>
      </div>
    </div>
    <div ref="verifiedMod" class="js-verifiedMod"></div>

  </div>
</template>

<script>
import $ from 'jquery';
import VerifiedMod from '../../../../backbone/views/components/VerifiedMod';
import { getModeratorOptions } from '@/utils/verifiedMod';
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
      _state: {
        maxPeerIDLength: 8,
        showAvatar: false,
      }
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
        ...this.model.toJSON(),
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit({
        ...options,
        initialState: {
          maxPeerIDLength: 8,
          showAvatar: false,
          ...options.initialState,
        },
      });

      this.verifiedModModel = app.verifiedMods.get(this.peerID);

      this.listenTo(app.verifiedMods, 'update', () => {
        const newVerifiedModModel = app.verifiedMods.get(this.peerID);
        if (newVerifiedModModel !== this.verifiedModModel) {
          this.verifiedModModel = newVerifiedModModel;
          this.render();
        }
      });
    },

    render () {
      const verifiedMod = app.verifiedMods.get(this.peerID);
      const createOptions = getModeratorOptions({
        model: verifiedMod,
      });

      if (!verifiedMod) {
        createOptions.initialState.tipBody = app.polyglot.t('verifiedMod.modUnverified.tipBodyOrderDetail', {
          not: `<b>${app.polyglot.t('verifiedMod.modUnverified.not')}</b>`,
          name: `<b>${app.verifiedMods.data.name}</b>`,
        });
      }

      if (this.verifiedMod) this.verifiedMod.remove();
      this.verifiedMod = this.createChild(VerifiedMod, createOptions);
      $(this.$refs.verifiedMod).append(this.verifiedMod.render().el);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
