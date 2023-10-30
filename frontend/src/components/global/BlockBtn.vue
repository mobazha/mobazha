<template>
  <div>
    <ProcessingButton
      :className="`clrP clrBr ${ob.useIcon && 'iconBtnSm tx3' || 'btn'} ${ob.isBlocking ? 'processing' : ''} ${ob.tooltipClass}`"
      @click.stop="onClickBlock"
      :data-tip="ob.isBlocked ? ob.polyT('blockButton.tipUnblock') : ob.polyT('blockButton.tipBlock')"
      :textClassName="textClassName"
      :btnText="btnText"
    />
  </div>
</template>

<script>
import app from '../../../backbone/app';
import * as block from '../../../backbone/utils/block';
import { recordEvent } from '../../../backbone/utils/metrics';


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
        ...this._state,
      }
    },
    textClassName () {
      const ob = this.ob;
      const textClassName = ob.isBlocked ? 'ion-eye' : 'ion-eye-disabled';
      return ob.useIcon ? textClassName : '';
    },
    btnText () {
      const ob = this.ob;
      const btnText = ob.isBlocked ? ob.polyT('blockButton.btnTxtUnblock') : ob.polyT('blockButton.btnTxtBlock');
      return ob.useIcon ? '' : btnText;
    }
  },
  methods: {
    loadData (options = {}) {
      if (typeof options.targetID !== 'string') {
        throw new Error('Please provide a targetID as a string.');
      }

      if (app.profile.id === options.targetID) {
        throw new Error('Blocking is not available on your own node.');
      }

      const opts = {
        ...options,
        initialState: {
          useIcon: false,
          tooltipClass: options.initialState && options.initialState.useIcon ?
            'toolTipNoWrap toolTipTop' : '',
          isBlocking: block.isBlocking(options.targetID) ||
            block.isUnblocking(options.targetID),
          isBlocked: block.isBlocked(options.targetID),
          ...(options && options.initialState || {}),
        },
      };

      this.baseInit(opts);
      this.targetID = options.targetID;

      this.listenTo(block.events, 'unblocking blocking', data => {
        if (!data.peerIDs.includes(options.targetID)) return;
        this.setState({ isBlocking: true });
      });

      this.listenTo(block.events, 'blocked unblocked blockFail unblockFail',
        data => {
          if (!data.peerIDs.includes(options.targetID)) return;
          this.setState({ isBlocking: false });
        });

      this.listenTo(block.events, 'blocked', data => {
        if (!data.peerIDs.includes(options.targetID)) return;
        this.setState({ isBlocked: true });
      });

      this.listenTo(block.events, 'unblocked', data => {
        if (!data.peerIDs.includes(options.targetID)) return;
        this.setState({ isBlocked: false });
      });
    },

    onClickBlock () {
      if (this.getState().isBlocked) {
        block.unblock(this.targetID);
        recordEvent('UnBlockUser');
      } else {
        block.block(this.targetID);
        recordEvent('BlockUser');
      }
    },

  }
}
</script>
<style lang="scss" scoped></style>
