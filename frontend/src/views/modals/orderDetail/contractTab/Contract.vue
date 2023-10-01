<template>
  <section>
    <template v-if="info.heading">
      <h1 class="tx4 txB row">{{ info.heading }}</h1>
    </template>
    <template v-if="info.errors && info.errors.length">
      <p class="txUnl rowSm clrTErr"><span class="ion-alert-circled padSm"></span>{{ ob.polyT('orderDetail.contractTab.contractErrorHeading') }}</p>
      <ul class="row">
        <li v-for="(err, j) in info.errors" :key="j" class="clrTErr rowSm">${err}</li>
      </ul>
    </template>
    <div class="border clrBr clrP clrT rowLg js-jsonContractContainer" @click.stop></div>

    <div class="flexHRight">
      <div class="posR">
        <a class="js-copyContract clrTEm" @click="onClickCopyContract">{{ ob.polyT(`orderDetail.contractTab.copyContract`) }}</a>
        <a class="copied js-copyContractDone clrT2">{{ ob.polyT(`orderDetail.contractTab.copyContractDone`) }}</a>
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
      info: {},
    };
  },
  created () {
    this.loadData(this.$props.options);
  },
  mounted () {
    this.render();
  },
  computed: {
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

      if (!options.contract) {
        throw new Error('Please provide a contract.');
      }

      this.contract = options.contract;
      this.info = opts;
    },

    onClickCopyContract () {
      ipc.send('controller.system.writeToClipboard', JSON.stringify(this.contract, null, 2));
      // Fade the link and make it unclickable, but maintain its position in the DOM.
      $('.js-copyContract')
        .addClass('unclickable')
        .velocity('stop')
        .velocity({ opacity: 0 })
        .velocity({ opacity: 1 }, {
          delay: 5000,
          complete: (els) => {
            $(els[0]).removeClass('unclickable');
          },
        });
      $('.js-copyContractDone')
        .velocity('stop')
        .velocity({ opacity: 1 })
        .velocity({ opacity: 0 }, { delay: 5000 });
    },

    render () {
      let renderjsonEl;

      if (this.rendered) {
        // On re-renders, reuse the renderjson el so it's state (e.g.
        // what is expanded/collapsed) is maintained.
        renderjsonEl = $('.js-jsonContractContainer').children()[0];
      }

      $('.js-jsonContractContainer').html(renderjsonEl || renderjson.set_show_to_level('1')(this.contract));

      this.rendered = true;
      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
