<template>
  <div>
    <ProcessingButton
      :className="`clrP clrBr ${useIcon && 'iconBtnSm tx3' || 'btn'} ${ob.isBlocking ? 'processing' : ''} ${ob.tooltipClass}`"
      @click="onClickBlock"
      @click.stop
      :attrs="{ 'data-tip': ob.isBlocked ? ob.polyT('blockButton.tipUnblock') : ob.polyT('blockButton.tipBlock') }"
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
  },
  data () {
    return {
      options: {},
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props);
  },
  mounted () {
  },
  computed: {
    textClassName () {
      const textClassName = this.isBlocked ? 'ion-eye' : 'ion-eye-disabled';
      return this.useIcon ? textClassName : '';
    },
    btnText () {
      const btnText = ob.isBlocked ? ob.polyT('blockButton.btnTxtUnblock') : ob.polyT('blockButton.btnTxtBlock');
      return ob.useIcon ? '' : btnText;
    }
  },
  methods: {
    loadData (options = {}) {
      if (typeof options.targetId !== 'string') {
        throw new Error('Please provide a targetId as a string.');
      }

      if (app.profile.id === options.targetId) {
        throw new Error('Blocking is not available on your own node.');
      }

      const opts = {
        ...options,
        initialState: {
          useIcon: false,
          tooltipClass: options.initialState && options.initialState.useIcon ?
            'toolTipNoWrap toolTipTop' : '',
          isBlocking: block.isBlocking(options.targetId) ||
            block.isUnblocking(options.targetId),
          isBlocked: block.isBlocked(options.targetId),
          ...(options && options.initialState || {}),
        },
      };

      super(opts);
      this.targetId = options.targetId;

      this.listenTo(block.events, 'unblocking blocking', data => {
        if (!data.peerIDs.includes(options.targetId)) return;
        this.setState({ isBlocking: true });
      });

      this.listenTo(block.events, 'blocked unblocked blockFail unblockFail',
        data => {
          if (!data.peerIDs.includes(options.targetId)) return;
          this.setState({ isBlocking: false });
        });

      this.listenTo(block.events, 'blocked', data => {
        if (!data.peerIDs.includes(options.targetId)) return;
        this.setState({ isBlocked: true });
      });

      this.listenTo(block.events, 'unblocked', data => {
        if (!data.peerIDs.includes(options.targetId)) return;
        this.setState({ isBlocked: false });
      });
    },

    onClickBlock () {
      if (this.getState().isBlocked) {
        block.unblock(this.targetId);
        recordEvent('UnBlockUser');
      } else {
        block.block(this.targetId);
        recordEvent('BlockUser');
      }
    },

  }
}
</script>
<style lang="scss" scoped></style>
