<template>
  <div class="configuration" @click="onDocumentClick">
    <div class="flexVCent clrT">
      <div :class="`col4 flexVCent gutterHTn ${ob.status === 'connected' ? 'txB' : ''}`">
        <template v-if="ob.status === 'connected'">
          <span class="ion-ios-checkmark-empty clrTEmph1 tx1"></span>
        </template>
        <div>{{ ob.name }}</div>
      </div>
      <div :class="`col4 ${ob.status === 'connected' ? 'txB' : ''}`">{{ ob.serverIp }}</div>
      <div class="col4">
        <div class="flexHRight">
          <div class="gutterHTn">
            <template v-if="!ob.builtIn">
              <a :class="`iconBtn clrP clrBr ion-trash-b js-btnDelete ${ob.deleteConfirmOn ? 'confirmDisabled' : ''}`" @click="onDeleteClick"></a>
            </template>
            <a class="iconBtn clrP clrBr ion-ios-gear " @click="onEditClick"></a>
            <template v-if="ob.status === 'connecting'">
              <a class="btn clrP clrBr  btnConnectCancel" @click="onCancelClick">
                {{ ob.spinner({ className: 'spinnerSm' }) }}
                {{ ob.polyT('connectionManagement.configurations.btnCancel') }}
              </a>
            </template>

            <template v-else-if="ob.status === 'connected'">
              <a class="btn clrP clrBr  btnDisconnect" @click="onDisconnectClick">{{
                ob.polyT('connectionManagement.configurations.btnDisconnect') }}</a>
            </template>

            <template v-else>
              <a class="btn clrP clrBr  btnConnectCancel" @click="onConnectClick">{{ ob.status ===
                'connect-attempt-failed' ? ob.polyT('connectionManagement.configurations.btnRetry') :
                ob.polyT('connectionManagement.configurations.btnConnect') }}</a>
            </template>
          </div>
        </div>
      </div>
    </div>
    <template v-if="ob.status === 'connect-attempt-failed'">
      <div class="errorBorder clrErr"></div>
    </template>
    <div class="deleteConfirm js-deleteConfirm arrowBoxTop border clrBr clrP clrT txCtr pad" v-show="!!ob.deleteConfirmOn" @click.stop.prevent>
      <div class="tx3 txB rowSm">{{ ob.polyT('connectionManagement.configurations.deleteConfirm.heading') }}</div>
      <p class="clrT2">{{ ob.polyT('connectionManagement.configurations.deleteConfirm.body') }}</p>
      <hr class="clrBr row" />

      <div class="flexHRight flexVCent gutterHLg">
        <a class="" @click="onDeleteConfirmCancel">{{ ob.polyT('connectionManagement.configurations.deleteConfirm.btnCancel') }}</a>
        <a class="btn clrP clrBr " @click="onDeleteConfirm">{{ ob.polyT('connectionManagement.configurations.deleteConfirm.btnDelete') }}</a>
      </div>
    </div>
  </div>
</template>

<script>
import _ from 'underscore';


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
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.model) {
        throw new Error('Please provide a server configuration model.');
      }

      this.baseInit(options);

      this._state = {
        status: 'not-connected',
        ...options.initialState || {},
      };

      this.listenTo(this.model, 'change', () => this.render());
    },

    onDocumentClick (e) {
      if (this.getState().deleteConfirmOn) {
        this.setState({ deleteConfirmOn: false });
      }
    },

    onConnectClick () {
      this.$emit('connectClick', { view: this });
    },

    onDisconnectClick () {
      this.$emit('disconnectClick', { view: this });
    },

    onCancelClick () {
      this.$emit('cancelClick', { view: this });
    },

    onEditClick () {
      this.$emit('editClick', { view: this });
    },

    onDeleteClick () {
      const isDeleteOn = this.getState().deleteConfirmOn;

      this.setState({ deleteConfirmOn: true });

      if (!isDeleteOn) {
        // If the delete confirm wasn't on, we will now show it
        // and we don't want this click event to bubble to our
        // document clieck handler, otherwise it will close the
        // confirm callout that we are showing here.
        return false;
      }

      return true;
    },

    onDeleteConfirm () {
      this.model.destroy();
      this.setState({ deleteConfirmOn: false });
    },

    onDeleteConfirmCancel () {
      this.setState({ deleteConfirmOn: false });
    },

    getState () {
      return this._state;
    },

    setState (state, replace = false) {
      let newState;

      if (replace) {
        this._state = {};
      } else {
        newState = _.extend({}, this._state, state);
      }

      if (!_.isEqual(this._state, newState)) {
        this._state = newState;
        this.render();
      }

      return this;
    },
  }
}
</script>
<style lang="scss" scoped></style>
