<template>
  <div class="statusMessageWrap">
    <div :class="`statusMessage statusMessage-${ob.type}`">
      <template v-if="ob.type === 'warning'">
        <span class="icon ion-alert-circled"></span>
      </template>

      <template v-else-if="ob.type === 'confirmed'">
        <span class="icon ion-ios-checkmark-empty"></span>
      </template>
      <div v-html="ob.msg" />
    </div>

  </div>
</template>

<script>
import { capitalize } from '../../backbone/utils/string';

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

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this.model.toJSON(),
      }
    },
  },
  methods: {
    loadData (options) {
      this.baseInit(options);
      this.listenTo(this.model, 'change', this.render);
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
          this.$emit(`click${capitalize(event)}`, { view: this, e });
        });
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
