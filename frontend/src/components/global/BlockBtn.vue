<template>
  <ProcessingButton
   :className="`clrP clrBr js-block ${useIcon && 'iconBtnSm tx3' || 'btn'} ${ob.isBlocking ? 'processing' : ''} ${ob.tooltipClass}`" ,
   :dataTip="tipText"
   :textClassName="textClassName"
   :btnText= "btnText"
  />
</template>

<script setup>
const props = defineProps({
  useIcon: Boolean,
  isBlocked: Boolean,
})

const tipText = isBlocked ? ob.polyT('blockButton.tipUnblock') : ob.polyT('blockButton.tipBlock');
let textClassName = isBlocked ? 'ion-eye' : 'ion-eye-disabled';
textClassName = useIcon ? textClassName : '';
let btnText = isBlocked ? ob.polyT('blockButton.btnTxtUnblock') : ob.polyT('blockButton.btnTxtBlock');
btnText = useIcon ? '' : btnText;
</script>
<style lang="scss" scoped>
</style>