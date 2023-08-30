<template>
  <div>
    <ProcessingButton
      :className="`clrP clrBr ${useIcon && 'iconBtnSm tx3' || 'btn'} ${ob.isBlocking ? 'processing' : ''} ${ob.tooltipClass}`" ,
      @click="onClickBlock"
      :dataTip="tipText"
      :textClassName="textClassName"
      :btnText= "btnText"
    />
  </div>
</template>

<script setup>
import app from '../../../backbone/app';
import * as block from '../../../backbone/utils/block';
import loadTemplate from '../../../backbone/utils/loadTemplate';
import { recordEvent } from '../../../backbone/utils/metrics';

const props = defineProps({
  useIcon: Boolean,
  isBlocked: Boolean,
})

const tipText = isBlocked ? ob.polyT('blockButton.tipUnblock') : ob.polyT('blockButton.tipBlock');
let textClassName = isBlocked ? 'ion-eye' : 'ion-eye-disabled';
textClassName = useIcon ? textClassName : '';
let btnText = isBlocked ? ob.polyT('blockButton.btnTxtUnblock') : ob.polyT('blockButton.btnTxtBlock');
btnText = useIcon ? '' : btnText;

loadData(props);

render();

function loadData (options = {}) {
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
}

function onClickBlock (e) {
  e.stopPropagation();

  if (this.getState().isBlocked) {
    block.unblock(this.targetId);
    recordEvent('UnBlockUser');
  } else {
    block.block(this.targetId);
    recordEvent('BlockUser');
  }
}


function render () {
  loadTemplate('components/blockBtn.html', (t) => {
    this.$el.html(t({
      ...this.getState(),
    }));
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
