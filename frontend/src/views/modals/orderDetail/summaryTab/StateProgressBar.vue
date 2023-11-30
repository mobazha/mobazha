<template>
  <div class="stateProgressBar">
    <template v-for="(state, index) in ob.states" :key="index">
      <div :class="`stateSection ${index < ob.currentState ? 'active' : ''}`" :style="`width: ${statesWidth(index)}%`">
        <div class="stateTrack"></div>
        <div class="stateCircle">
          <span class="ion-ios-checkmark-empty"></span>
        </div>
        <div class="stateCircleBorderFillIn"></div>
        <div class="stateLabel">{{ state }}</div>
        <template v-if="ob.disputeState === index + 1">
          <div class="disputeOpenedBadge clrBr clrP">
            <span class="ion-alert-circled"></span>
          </div>
        </template>
      </div>
    </template>

  </div>
</template>

<script>

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    barState: {
      type: Object,
      default: {
        states: ['Point 1', 'Point 2'],
        currentState: 0,
        disputeState: 0,
      },
    }
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
        states: ['Point 1', 'Point 2'],
        currentState: 0,
        disputeState: 0,
        ...this.barState,
      };
    }
  },
  methods: {
    statesWidth (index) {
      const ob = this.ob;
      let width = index === 0 || index === ob.states.length - 1 ?
        0.5 / (ob.states.length - 1) : 1 / (ob.states.length - 1);
      return width * 100;
    },
    loadData () {
      const state = {
        states: ['Point 1', 'Point 2'],
        currentState: 0,
        disputeState: 0,
        ...this.barState
      };

      if (!Array.isArray(state.states)) {
        throw new Error('Please provide an array of states.');
      }

      if (state.states.length < 2) {
        throw new Error('Please provide at least two states.');
      }

      if (typeof state.currentState !== 'number') {
        throw new Error('Please provide the current state as a number.');
      }

      // pass in 0 for an empty progress bar, otherwise integers above
      // zero correspond to the 1 based position in the states array
      if (state.currentState < 0 ||
        state.currentState > state.states.length) {
        throw new Error('The current state cannot be less than zero or greater then ' +
          'the length of the provided states array.');
      }

      if (typeof state.disputeState !== 'number') {
        throw new Error('Please provide the dispute state as a number.');
      }

      // pass in 0 to not show the disputed indicator, otherwise pass in
      // the state the dispute was opened in and the indicator will appear
      // half-way between that state and the following one.
      if (state.disputeState < 0 ||
        state.disputeState > state.states.length - 1) {
        throw new Error('The dispute state must be greater than 0 and less than ' +
          'the length of the state array minus one.');
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
