<template>
  <div :class="`filters rowHg`">
    <div v-for="(row, i) in rows" :key="i">
      <div class="flex gutterH gutterV">
        <div v-for="(filter, j) in row" :key="j">
          <div class="col3">
            <div :class="`filter clrP clrBr clrSh2 ${filter.className}`">
              <input type="checkbox" :id="filter.id" :checked="filter.checked"
                :data-state="JSON.stringify(filter.targetState)" v-bind="filter.attrs">
              <label class="tx5b" :for="filter.id">{{ filter.text }}</label>
            </div>
          </div>
        </div>

        <!-- // If necessary, add in spacers. -->
        <div v-for="k in (maxPerRowFinal - row.length)" :key="k">
          <div class="col3"></div>
        </div>
      </div>
    </div>
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
    maxPerRow: Number,
    filters: Object,
  },
  data () {
    return {
    };
  },
  computed: {
    maxPerRowFinal () {
      return ob.maxPerRow || 4;
    },
    rows () {
      return ob.splitIntoRows(this.filters, this.maxPerRowFinal);
    },
  },
  methods: {

  }
}
</script>
<style lang="scss" scoped></style>
