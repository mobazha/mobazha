<template>
  <div class="profileBox">
    <template v-if="!options.isFetching">
      <a :href="`#${options.peerID}`" :style="ob.getAvatarBgImage(info.avatarHashes,
        {
          standardSize: 'small',
          responsiveSize: 'medium',
        })" class="avatar clrBr2 clrSh1 disc"></a>
      <a :href="`#${options.peerID}`" class="txB clamp clrT">{{ info.name }}</a>
      <div class="clrT2 tx5 clamp">{{ info.location }}</div>
    </template>

  </div>
</template>

<script>
export default {
  mixins: [],
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      fetchFailed: false,
    };
  },
  created () {
  },
  mounted () {
  },
  computed: {
    info () {
      if (this.options.model && typeof this.options.model.toJSON === 'function') {
        return this.options.model.toJSON();
      }
      return {}
    },
  },
  methods: {
  }
}
</script>
<style lang="scss" scoped></style>
