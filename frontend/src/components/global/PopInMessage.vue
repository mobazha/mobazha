<template>
  <div class="popInMessage clrP border clrBrT pad tx5 clrSh1">
    <template v-if="ob.dismissable">
      <a class="closeIcon tx2 js-dismiss" @click="onClickDismiss">
        <span class="ion-ios-close-empty clrBr clrP clrT clrBrT"></span>
      </a>
    </template>
    <template v-if="ob.messageText">
      <p class="txUnl">
        <span class="ion-alert-circled"></span>
        <b>{{ ob.messageText }}</b>
        <a class="clrTEm js-refresh" @click="onClickRefresh">{{ ob.polyT('refreshAlertPopInMessage.refreshLink') }}</a>
      </p>
    </template>

    <template v-else>
      {{ ob.messageHTML }}
    </template>
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
  },
  data() {
    return {};
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
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
    loadData(options) {
      this.baseInit(options);

      const opts = {
        dismissable: false,
        ...options,
      };
      // Use messageText if you're providing inline content which the template
      // will wrap in a <p>. Otherwise, use messageHTML and provide the full markup.
      if (!options.messageText && !options.messageHTML) {
        throw new Error('Please provide a messageText or messageHTML');
      }

      if (options.messageText && options.messageHTML) {
        throw new Error('Please provide only one of messageText or messageHTML');
      }

      this._state = {
        ...opts,
        ...(opts.initialState || {}),
      };
    },

    onClickDismiss() {
      this.$emit('clickDismiss');
    },

    onClickRefresh() {
      this.$emit('clickRefresh');
    },

    setState(state = {}) {
      const newState = {
        ...this._state,
        ...state,
      };

      if (!_.isEqual(this._state, newState)) {
        this._state = newState;
      }
    },

    replaceState(state = {}) {
      if (!_.isEqual(this._state, state)) {
        this._state = state;
      }
    },
  },
};
</script>
<style lang="scss" scoped></style>
