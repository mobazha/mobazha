<template>
  <div class="feeChange">
    {{ ob.polyT('feeChangeWidget.label') }} <span :class="ob.feeLevelClass">{{ ob.polyT(`feeLevels.${ob.feeLevel}`) }}</span> <button :class="ob.changeLinkClass" @click="onClickChangeFee">{{ ob.polyT('feeChangeWidget.btnChange') }}</button>
  </div>
  <Teleport to="#js-vueModal">
    <Settings v-if="showSettings" :options="{ initialTab: 'Advanced', scrollTo: '.js-feeSection', }" @close="closeSettings" />
  </Teleport>
</template>

<script>
import app from '../../../backbone/app';
import Settings from '@/views/modals/settings/Settings.vue';

export default {
  components: {
    Settings,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data() {
    return {
      _state: {
        feeLevel: app.localSettings.get('defaultTransactionFee'),
        feeLevelClass: 'txB',
        changeLinkClass: 'btnAsLink clrT2',
      },

      showSettings: false,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
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

      this.baseInit(opts);

      this.listenTo(app.localSettings, 'change:defaultTransactionFee', (md, val) => this.setState({ feeLevel: val }));
    },

    onClickChangeFee() {
      this.showSettings = true;
    },

    closeSettings() {
      this.showSettings = false;
    }
  },
};
</script>
<style lang="scss" scoped></style>
