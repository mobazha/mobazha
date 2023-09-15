<template>
  <div class="feeChange">
    {{ ob.polyT('feeChangeWidget.label') }}
    <span :class="ob.feeLevelClass">{{ ob.polyT(`feeLevels.${ob.feeLevel}`) }}</span>
    <button :class="ob.changeLinkClass" @click="onClickChangeFee">{{ ob.polyT('feeChangeWidget.btnChange') }}</button>
  </div>
</template>

<script>
import app from '../../../backbone/app';
import { launchSettingsModal } from '../../../backbone/utils/modalManager';
import loadTemplate from '../../../backbone/utils/loadTemplate';


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
        ...this.getState(),
      };
    }
  },
	methods: {
  loadData(options = {}) {
    const opts = {
      initialState: {
        feeLevel: app.localSettings.get('defaultTransactionFee'),
        feeLevelClass: 'txB',
        changeLinkClass: 'btnAsLink clrT2',
      },
      ...options,
    };

    super(opts);

    this.listenTo(app.localSettings, 'change:defaultTransactionFee', (md, val) => this.setState({ feeLevel: val }));
  },

  onClickChangeFee() {
    launchSettingsModal({
      initialTab: 'Advanced',
      scrollTo: '.js-feeSection',
    });
  },

  render() {
    loadTemplate('components/feeChange.html', (t) => {
      this.$el.html(t({
        ...this.getState(),
      }));
    });

    return this;
  }

  }
}
</script>
<style lang="scss" scoped>
</style>
