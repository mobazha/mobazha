<template>
  <div class="profileBox">
    <template v-if="!ob.isFetching">
      <a :href="`#${ob.peerID}`" :style="ob.getAvatarBgImage(ob.avatarHashes,
        {
          standardSize: 'small',
          responsiveSize: 'medium',
        })" class="avatar clrBr2 clrSh1 disc"></a>
      <a :href="`#${ob.peerID}`" class="txB clamp clrT">{{ ob.name }}</a>
      <div class="clrT2 tx5 clamp">{{ ob.location }}</div>
    </template>

  </div>
</template>

<script>
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
        isFetching: false,
        peerID: '',
        fetchFailed: false,
      }
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
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
  methods: {
    loadData(options = {}) {
      const opts = {
        initialState: {
          isFetching: false,
          fetchFailed: false,
        },
        ...options,
      };

      this.baseInit(opts);
    },
  }
}
</script>
<style lang="scss" scoped></style>
