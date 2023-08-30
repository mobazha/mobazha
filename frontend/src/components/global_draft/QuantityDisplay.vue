<template>
  <div>

    <span v-if="typeof ob.amount === 'number'" class="content {{ ob.contentClass }}">{{ formattedAmount }}</span>
    <div v-else-if="ob.isFetching">
      <SpinnerSVG :className="ob.spinnerClass" />
    </div>
    <div v-else-if="ob.fetchFailed" class="content {{ ob.contentFailedClass }}">
      <div class="arrowBoxTipWrap {{ ob.tipClass }}">
        <div class="flexVCent gutterHSm">
          <i class="clrT2">Unknown</i>
          <i class="ion-help-circled"></i>
        </div>

        <div v-if="ob.fetchError" class="arrowBoxCenteredTop clrBr clrP">{{ message }}</div>
      </div>
    </div>

  </div>
</template>

<script setup>
const props = defineProps({
  phase: String,
})

const formattedAmount = computed(() => {
  let formattedAmount = new Intl.NumberFormat(ob.localCur, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 4,
  }).format(ob.amount);

  formattedAmount = ob.coinType ?
    ob.polyT('cryptoAmountIconPairing', {
      amount: `<span>${formattedAmount}</span>`,
      icon: ob.crypto.cryptoIcon({ code: ob.coinType }),
    }) :
    formattedAmount;

  return formattedAmount;
});

const retryLink = `<a class="js-retry">${ob.polyT('inventoryDisplay.retryLink')}</a>`;
let message = ob.polyT('inventoryDisplay.fetchError', {
  retryLink,
});

if (ob.fetchError) {
  message = ob.polyT('inventoryDisplay.fetchErrorWithMsg', {
    msg: ob.fetchError,
  });
  message += `<br /> <br /><div class="txCtr">${retryLink}</div>`;
}

</script>
<style lang="scss" scoped>
</style>