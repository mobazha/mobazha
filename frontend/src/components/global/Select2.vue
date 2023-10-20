<template>
  <select>
    <slot></slot>
  </select>
</template>
<script>
import $ from 'jquery';

export default {
  props: ["options", "modelValue"],
  emits: ['update:modelValue', 'change'],
  computed: {
    value: {
      get() {
        return this.modelValue;
      },
    }
  },
  mounted () {
    var vm = this;
    $(this.$el)
      // init select2
      .select2( this.options ).val(this.value).trigger("change")
      // emit event on change.
      .on("change", function (event) {
        vm.$emit('update:modelValue', this.value);
        vm.$emit('change', event);
      });
  },
  unmounted() {
    $(this.$el).off().select2("destroy");
  }
};
</script>
<style lang="scss" scoped></style>