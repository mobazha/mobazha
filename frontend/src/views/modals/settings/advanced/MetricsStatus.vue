<template>
  <div class="flexVCent gutterHMd ">
    <a class="btn clrP clrBr clrSh2 flexNoShrink" @click="onClickChangeSharing">{{
      ob.polyT('settings.advancedTab.integrations.changeBtn') }}</a>
    <div class="flexExpand txLft">
      <template v-if="ob.restartRequired">
        <span class="clrTEmph1">{{ ob.polyT('metrics.sharingWillBeOn') }}</span>
      </template>

      <template v-else-if="ob.shareMetrics">
        <span class="clrTEmph1">{{ ob.polyT('metrics.sharingIsOn') }}</span>
      </template>

      <template v-else>
        <span class="clrTErr">{{ ob.polyT('metrics.sharingIsOff') }}</span>
      </template>
    </div>

  </div>
</template>

<script>
import app from '../../../../../backbone/app';
import loadTemplate from '../../../../../backbone/utils/loadTemplate';
import { showMetricsModal, isMetricRestartNeeded, recordEvent } from '../../../../../backbone/utils/metrics';

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
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        shareMetrics: app.localSettings.get('shareMetrics'),
        restartRequired: isMetricRestartNeeded(),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this.listenTo(app.localSettings, 'change:shareMetrics', () => this.render());
    },

    onClickChangeSharing () {
      recordEvent('Settings_Advanced_ChangeSharing');
      showMetricsModal();
    },

  }
}
</script>
<style lang="scss" scoped></style>
