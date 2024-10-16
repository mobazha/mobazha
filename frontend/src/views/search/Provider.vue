<template>
  <div :class="`searchProvider flexVCent clrBrT ${ob.active ? 'active' : ''}`">
    <button
      :class="`clrP clrBr clrSh2 providerBtn ${ob.showSelectDefault ? 'showSelectDefault' : ''} ${ob.name ? 'toolTipNoWrap' : ''}`"
      @click="onClickProvider" :data-tip="ob.name">
      <img class="thumb providerInner" :src="ob.logo ? ob.logo : (ob.localLogo ? ob.localLogo : ob.getImagePath('defaultProvider.png'))" />
    </button>
  </div>
</template>

<script>
import ProviderMd from '../../../backbone/models/search/SearchProvider';
import { recordEvent } from '../../../backbone/utils/metrics';

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

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        ...this.options,
      }
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.model || !(options.model instanceof ProviderMd)) {
        throw new Error('Please provide a model.');
      }

      this.baseInit(options);
    },

    onClickProvider () {
      this.$emit('click', this.model);
      recordEvent('Discover_ChangeProvider', {
        name: this.model.get('name') || 'unknown',
        url: this.model.get('listings'),
      });
    },
  }
}
</script>
<style lang="scss" scoped></style>
