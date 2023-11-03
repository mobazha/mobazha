<template>
  <div class="modal messageModal dialog">
    <BaseModal>
      <template v-slot:component>
        <div class="contentBox clrP clrBr clrSh3">
          <div class="padMd">
            <div class="">
              <h1>{{ ob.polyT('startUp.dialogs.unableToGetVerifiedMods.title') }}</h1>
            </div>
            <div class="">
              <div class="rowMd innerMessage">
                {{ ob.polyT('startUp.dialogs.unableToGetVerifiedMods.msg', { reason: ob.reason }) }}
              </div>
            </div>
          </div>
          <div class="flexRow flexBtnWrapper">
            <ProcessingButton
              :className="`flexExpand btnFlx clrP js-retry ${ob.fetching ? 'processing' : ''}`"
              @click="onRetryClick"
              :btnText="ob.polyT('startUp.dialogs.unableToGetVerifiedMods.btnRetry')" />
            <a class="flexExpand btnFlx clrP js-continue" @click="onContinueClick">{{ ob.polyT('startUp.dialogs.unableToGetVerifiedMods.btnClose') }}</a>
          </div>
        </div>
      </template>
    </BaseModal>
  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        fetching: false,
        reason: '',
      }
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
        ...this._state,
      }
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        removeOnClose: true,
        ...options,
        initialState: {
          fetching: false,
          reason: '',
          ...(options.initialState || {}),
        },
      };

      this.baseInit(opts);
    },

    onRetryClick () {
      this.$emit('retry');
    },

    onContinueClick () {
      this.$emit('continue');
      this.close();
    },
  }
}
</script>
<style lang="scss" scoped></style>
