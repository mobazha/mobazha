<template>
  <section>
    <template v-if="ob.heading">
      <h1 class="tx4 txB row">{{ ob.heading }}</h1>
    </template>
    <template v-if="ob.errors && ob.errors.length">
      <p class="txUnl rowSm clrTErr"><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.contractTab.contractErrorHeading') }}</p>
      <ul class="row">
        <li v-for="(err, j) in ob.errors" :key="j" class="clrTErr rowSm">{{ err }}</li>
      </ul>
    </template>
    <div ref="jsonContractContainer" class="border clrBr clrP clrT rowLg js-jsonContractContainer" @click.stop.prevent></div>

    <div class="flexHRight">
      <div class="posR">
        <a ref="copyContract" class="js-copyContract clrTEm" @click="onClickCopyContract">{{ ob.polyT(`orderDetail.contractTab.copyContract`) }}</a>
        <a ref="copyContractDone" class="copied js-copyContractDone clrT2">{{ ob.polyT(`orderDetail.contractTab.copyContractDone`) }}</a>
      </div>
    </div>

  </section>
</template>

<script>
import $ from 'jquery';
import { ipc } from '../../../../utils/ipcRenderer.js';
import renderjson from '../../../../../backbone/lib/renderjson.js';
import 'velocity-animate';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        heading: '',
        errors: [],
      }
    };
  },
  created () {
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
      }
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        ...options,
        initialState: {
          heading: '',
          errors: [],
          ...options.initialState || {},
        },
      };
      this.baseInit(opts);

      if (!options.contract) {
        throw new Error('Please provide a contract.');
      }
    },

    onClickCopyContract () {
      ipc.send('controller.system.writeToClipboard', JSON.stringify(this.contract, null, 2));
      // Fade the link and make it unclickable, but maintain its position in the DOM.
      $(this.$refs.copyContract)
        .addClass('unclickable')
        .velocity('stop')
        .velocity({ opacity: 0 })
        .velocity({ opacity: 1 }, {
          delay: 5000,
          complete: (els) => {
            $(els[0]).removeClass('unclickable');
          },
        });
      $(this.$refs.copyContractDone)
        .velocity('stop')
        .velocity({ opacity: 1 })
        .velocity({ opacity: 0 }, { delay: 5000 });
    },

    render () {
      let renderjsonEl;

      if (this.rendered) {
        // On re-renders, reuse the renderjson el so it's state (e.g.
        // what is expanded/collapsed) is maintained.
        renderjsonEl = $(this.$refs.jsonContractContainer).children()[0];
      }

      $(this.$refs.jsonContractContainer).html(renderjsonEl || renderjson.set_show_to_level('1')(this.contract));

      this.rendered = true;
      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
