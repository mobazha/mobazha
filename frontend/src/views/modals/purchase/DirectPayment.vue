<template>
  <div>
    <!-- // duplicate the moderator card html to make sure everything aligns -->

    <div class="moderatorCard clrBrInvis clickable " @click="clickDirectPurchase">
      <div class="moderatorCardInner">
        <div class="flexRow gutterH moderatorCardContent">
          <div class="flexNoShrink">
            <div class="btnRadio">
              <div tabindex="0" :class="`fauxRadioBtn ${ob.active ? 'active' : ''}`"></div>
            </div>
          </div>
          <div class="moderatorCardMiddle">
            <div class="flex rowSm">
              <b>{{ ob.polyT('purchase.directPayment') }}</b>
            </div>
            <div class="clrT2">{{ ob.polyT('purchase.directPaymentDetails') }}</div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup>
import loadTemplate from '../../../../backbone/utils/loadTemplate';

const props = defineProps({
  phase: String,
  outdatedHash: String,
})

loadData(props);

render();

function loadData (options = {}) {
  const opts = {
    className: 'moderatorsWrapper fauxModeratorsWrapper',
    ...options,
    initialState: {
      active: false,
      ...options.initialState || {},
    },
  };

  super(opts);
}

function clickDirectPurchase () {
  this.setState({ active: true });
  this.trigger('click', { active: true });
}

function render () {
  loadTemplate('modals/purchase/directPayment.html', t => {
    this.$el.html(t({
      ...this.getState(),
    }));

    super.render();
  });

  return this;
}

</script>
<style lang="scss" scoped>
</style>
