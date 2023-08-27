<template>
  <div>
    <div class="flexVCent">
      <h2 class="h4 required flexExpand">{{ ob.polyT('purchase.shippingTitle') }}</h2>
      <a class="clrTEm txU tx5b js-newAddress" v-if="ob.userAddresses.length">{{ ob.polyT('purchase.newAddress') }}</a>
    </div>
    <div class="row">
      <select id="shippingAddress" v-if="ob.userAddresses.length">
        <option v-for="(a, i) in ob.userAddresses" :key="i" :value="i" :selected="ob.selectedAddressIndex === i">
          {{ getAddress(a) }}
        </option>
      </select>
      <div class="padGi txCtr" v-else>
        <div class="txB row">
          {{ ob.polyT('purchase.noAddresses') }}
        </div>
        <button class="btn clrP clrBr js-newAddress">
          {{ ob.polyT('purchase.newAddress') }}
        </button>
      </div>

    </div>
    <div class="js-shippingOptionsWrapper"></div>

  </div>
</template>

<script setup>

function getAddress (a) {
  const addr = [];
  addr.push(a.name);
  if (a.company) addr.push(a.company);
  if (a.addressLineOne) addr.push(a.addressLineOne);
  if (a.addressLineTwo) addr.push(a.addressLineTwo);
  if (a.city) addr.push(a.city);
  const state = a.state || '';
  const code = a.postalCode || '';
  const stateCode = `${state ? `${state} ` : ''}${code}`;
  if (stateCode) addr.push(stateCode);

  return addr.join(', ');
}

</script>
<style lang="scss" scoped>
</style>