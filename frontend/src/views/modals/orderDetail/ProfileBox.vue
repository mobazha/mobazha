<template>
  <div class="profileBox">
    <div v-if="!isFetching">
      <a :href="`#${model.peerID}`" :style="ob.getAvatarBgImage(model.avatarHashes,
        {
          standardSize: 'small',
          responsiveSize: 'medium',
        })" class="avatar clrBr2 clrSh1 disc"></a>
      <a :href="`#${model.peerID}`" class="txB clamp clrT">{{ model.name }}</a>
      <div class="clrT2 tx5 clamp">{{ model.location }}</div>
    </div>

  </div>
</template>

<script>
export default {
  mixins: [],
  data () {
    return {
      isFetching: false,
      fetchFailed: false,

      model,
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
    this.render();
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      if (this.model) this.setModel(this.model);
    },

    setModel (md) {
      this.model = md;
    },
  }
}
</script>
<style lang="scss" scoped></style>
