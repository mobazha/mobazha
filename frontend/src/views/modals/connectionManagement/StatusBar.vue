<template>
  <div class="statusBarMessage ">
    <div class="pad posR clrP flexVCent <% if (ob.status === 'connect-attempt-failed') print('clrBrError connectFailed') %>">
      <p v-if="ob.status === 'connecting'" class="txUnl tx5 statusMsg connectingMsg clamp3">
        <SpinnerSVG className="spinnerSm posA" />
        {{ ob.msg }}
      </p>
      <p v-else class="txUnl tx5 statusMsg clamp3">{{ ob.msg }}</p>
      <a class="btnClose" @cick="onClickClose" ><span class="ion-ios-close-empty"></span></a>
    </div>
  </div>
</template>

<script>
import _ from 'underscore';
import { capitalize } from '../../../../backbone/utils/string';

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
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this._state = {
        msg: '',
        ...options.initialState || {},
      };
    },

    className () {
      return 'statusBarMessage ';
    },

    events () {
      return {
        'click .statusMsg [class^="js-"], .statusMsg [class*=" js-"]': 'onMsgContentClick',
      };
    },

    onClickClose () {
      this.$emit('closeClick');
    },

    // User of this view may embed CTA's in the msg. This is a generic handler to
    // trigger events for them.
    onMsgContentClick (e) {
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
