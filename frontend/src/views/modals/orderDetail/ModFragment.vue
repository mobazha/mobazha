<template>
  <div>
    <div class="flexVCent gutterHSm rowTn">
      <div v-if="modInfo.showAvatar">
        <div class="avatar clrBr2 clrSh1 disc" :style="ob.getAvatarBgImage(modInfo.avatarHashes || {})"></div>
      </div>
      <div>{{ modInfo.name }}
        <a class="clrTEm" :href="`#${modInfo.peerID}`">{{ modInfo.handle && `@${modInfo.handle}` || `${modInfo.peerID.slice(0,
          modInfo.maxPeerIDLength)}…` }}</a>
      </div>
    </div>
    <div class="js-verifiedMod"></div>

  </div>
</template>

<script>
import VerifiedMod, { getModeratorOptions } from '../../../../backbone/views/components/VerifiedMod';
import app from '../../../../backbone/app';


export default {
  props: {
  },
  data () {
    return {
      modInfo: {
        showAvatar: false,
        avatarHashes: [],
        name: '',
        peerID: '',
        handle: '',
        maxPeerIDLength: 8,
      }
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
  },
  methods: {
    loadData (options = {}) {

      this.options = options;
      this.verifiedModModel = app.verifiedMods.get(this.modInfo.peerID);

      this.listenTo(app.verifiedMods, 'update', () => {
        const newVerifiedModModel = app.verifiedMods.get(this.modInfo.peerID);
        if (newVerifiedModModel !== this.verifiedModModel) {
          this.verifiedModModel = newVerifiedModModel;
          this.render();
        }
      });
    },

    render () {

      const verifiedMod = app.verifiedMods.get(this.modInfo.peerID);
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
      this.getCachedEl('.js-verifiedMod').append(this.verifiedMod.render().el);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
