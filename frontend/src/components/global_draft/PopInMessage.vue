<template>
  <div class="popInMessage clrP border clrBrT pad tx5 clrSh1">
    <div v-if="ob.dismissable">
      <a class="closeIcon tx2 js-dismiss">
        <span class="ion-ios-close-empty clrBr clrP clrT clrBrT"></span>
      </a>
    </div>
    <div v-if="ob.messageText">
      <p class="txUnl">{{ ob.messageText }}</p>
    </div>

    <div v-else>
      {{ ob.messageHTML }}
    </div>

  </div>
</template>

<script>
import _ from 'underscore';
import { capitalize } from '../../../backbone/utils/string';
import app from '../../../backbone/app';
import loadTemplate from '../../../backbone/utils/loadTemplate';
import baseVw from '../baseVw';


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
  },
  methods: {
    loadData (options) {
      super(options);

      const opts = {
        dismissable: false,
        ...options,
      };

      this.options = opts;

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

    className () {
      return 'popInMessage clrP border clrBrT pad tx5 clrSh1';
    },

    events () {
      return {
        'click [class^="js-"], [class*=" js-"]': 'onClick',
      };
    },

    onClick (e) {
      // If the the el has a '.js-<class>' class, we'll trigger a
      // 'click<Class>' event from this view.
      const events = [];

      e.currentTarget.classList.forEach((className) => {
        if (className.startsWith('js-')) events.push(className.slice(3));
      });

      if (events.length) {
        events.forEach(event => {
          this.trigger(`click${capitalize(event)}`, { view: this, e });
        });
      }
    },

    setState (state = {}) {
      const newState = {
        ...this._state,
        ...state,
      };

      if (!_.isEqual(this._state, newState)) {
        this._state = newState;
        this.render();
      }
    },

    replaceState (state = {}) {
      if (!_.isEqual(this._state, state)) {
        this._state = state;
        this.render();
      }
    },

    render () {
      loadTemplate('./components/popInMessage.html', (tmpl) => {
        this.$el.html(
          tmpl(this._state)
        );
      });

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
