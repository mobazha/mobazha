<template>
  <div :class="`filters rowHg ${ob.className}`">
    <template v-for="(row, i) in rows" :key="i">
      <div class="flex gutterH gutterV">
        <div class="col3" v-for="(filter, j) in row" :key="j">
          <div :class="`filter clrP clrBr clrSh2 ${filter.className}`">
            <input type="checkbox"
              :id="filter.id"
              :checked="filter.checked"
              v-bind="filter.attrs"
              v-model="checkResult[`${i}_${j}`]"
              @change="onChangeFilter(filter, checkResult[`${i}_${j}`])"
              >
            <label class="tx5b" :for="filter.id">{{ filter.text }}</label>
          </div>
        </div>

        <!-- // If necessary, add in spacers. -->
        <template v-for="k in (maxPerRow - row.length)" :key="k">
          <div class="col3"></div>
        </template>
      </div>
    </template>
  </div>
</template>

<script>

/*
  pass in the following data structure:

  {
    className: 'top-level-class', // optional
    maxPerRow: 3, // optional, deafult is 4
    filters: [
      {
        className: 'filterFulfilled', // optional
        id: 'filterFulfilled',
        text: 'Fulfilled',
        checked: true,
        // This corresponds to the transaction server state(s) that the
        // filter is correlated with.
        targetState: [0, 1],
        attrs: { // optional
          happy: 'yes',
          jovial: 'certainly',
        },
      },
      ...
    ]
  }
*/

export default {
  props: {
    filters: {
      type: Object,
      default: {},
	  },
    className: {
      type: String,
      default: {},
	  },
    maxPerRow: {
      type: Number,
      default: 4,
    }
  },
  data () {
    return {
      checkResult: {},
    };
  },
  created () {
  },
  computed: {
    rows () {
      return this.ob.splitIntoRows(this.filters, this.maxPerRow);
    },
  },
  methods: {
    onChangeFilter(filter, checked) {
      this.$emit('changeFilter', filter, checked);
    }
  }
}
</script>
<style lang="scss" scoped></style>
