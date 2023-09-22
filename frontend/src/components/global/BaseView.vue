<template>
  <div>
    <div id="contentFrameVue" class="clrBr">
      <div id="pageContainerVue">
        <slot name="component"></slot>
      </div>
    </div>
    <slot name="modal"/>
  </div>
</template>

<script>
import _ from 'underscore';

export default {
  props: {
  },
  created() {
  },
  data () {
    return {
     
    }
  },
  watch: {
    $route (to, from) {

    }
  },
  mounted() {

  },
  methods: {
    
  }
}
</script>