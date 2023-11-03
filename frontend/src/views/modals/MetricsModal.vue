<template>
  <div class="modal messageModal dialog">
    <BaseModal>
      <template v-slot:component>
        <div class="contentBox tx5b padLg clrP clrBr clrSh3 dialogScrollMsg">
          <div class="flexRow">
            <h1 class="flexExpand flexNoShrink">{{ ob.polyT('metrics.title') }}</h1>
            <span class="flexHRight clrT2">v{{ ob.mVersion }}</span>
          </div>
          <div class="rowMd innerMessage">
            <div class="rowHg">
              <template v-if="ob.showNewMessage">
                <p class="clrTEm">
                  <b>{{ ob.polyT('metrics.newVersion') }}</b>
                </p>
              </template>
              <p>{{ ob.polyT('metrics.msg1') }}</p>
              <p class="tx6"><i>{{ ob.polyT('metrics.msg2') }}</i></p>
            </div>
            <h4>{{ ob.polyT('metrics.listTitle') }}</h4>
            <ul class="tx6 padSmKids rowLg">
              <li>{{ ob.polyT('metrics.list.errors') }}</li>
              <li>{{ ob.polyT('metrics.list.user') }}</li>
              <li>{{ ob.polyT('metrics.list.node') }}</li>
              <li>{{ ob.polyT('metrics.list.listings') }}</li>
              <li>{{ ob.polyT('metrics.list.orders') }}</li>
              <li>{{ ob.polyT('metrics.list.app') }}</li>
              <li>{{ ob.polyT('metrics.list.system') }}</li>
              <li>{{ ob.polyT('metrics.list.path') }}</li>
            </ul>
            <div class="tx6">
              <p><i>{{ ob.polyT('metrics.disclaimer1') }}</i></p>
              <p><i>{{ ob.polyT('metrics.disclaimer2') }}</i></p>
            </div>
          </div>
          <div class="flexVCent flexHRight gutterH">
            <template v-if="ob.shareMetrics !== undefined">
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
            </template>
            <template v-if="ob.showUndecided">
              <a class="btn clrP clrBr " @click="onDeclineClick">{{ ob.polyT('metrics.declineBtn') }}</a>
              <a class="btn clrP clrBr " @click="onShareClick">{{ ob.polyT('metrics.acceptBtn') }}</a>
            </template>

            <template v-else>
              <a class="btn clrP clrBr" :disabled="!ob.shareMetrics" @click="onDeclineClick">{{
                ob.polyT('metrics.deactivateBtn') }}</a>
              <a class="btn clrP clrBr" :disabled="ob.shareMetrics" @click="onShareClick">{{
                ob.polyT('metrics.activateBtn') }}</a>
            </template>
          </div>
        </div>

      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../backbone/app';
import {
  changeMetrics,
  isMetricRestartNeeded,
  mVersion,
  isNewerVersion
} from '../../../backbone/utils/metrics';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      showUndecided: false,
      shareMetrics: false,
      restartRequired: false,
      mVersion: '',
      showNewMessage: false,
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
        showUndecided: this.showUndecided,
        shareMetrics: app.localSettings.get('shareMetrics'),
        restartRequired: isMetricRestartNeeded(),
        mVersion,
        showNewMessage: isNewerVersion(),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this.listenTo(app.localSettings, 'change:shareMetrics', () => this.shareMetrics = app.localSettings.get('shareMetrics'));
    },

    onShareClick () {
      changeMetrics(true)
        .done(() => {
          this.close();
        })
        .fail(() => {
          // the save is to local storage, this shouldn't happen
          console.log('Saving shareMetrics as true has failed.');
        });
    },

    onDeclineClick () {
      changeMetrics(false)
        .done(() => {
          this.close();
        })
        .fail(() => {
          // the save is to local storage, this shouldn't happen
          console.log('Saving shareMetrics as false has failed.');
        });
    },

    close () {
      this.$emit('close');
    }
  }
}
</script>
<style lang="scss" scoped></style>
