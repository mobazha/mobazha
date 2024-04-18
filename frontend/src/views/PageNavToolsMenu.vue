<template>
  <div class="listBox clrBr clrP clrSh1">
    <div class="listGroup clrP clrBr">
      <a class="listItem js-navListItem" @click="onWooImporter">
        <span>{{ ob.polyT('pageNav.toolsMenu.wooImporter') }}</span>
      </a>
    </div>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import app from '../../backbone/app';

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
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      
    },
    onWooImporter() {
      this.$emit('onWooImporterClick');
    }
  }
}
</script>
<style lang="scss" scoped></style>
