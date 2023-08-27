<template>
  <div>
    <div class="moderatorsWrapper clrBr <% print(ob.wrapperClasses) %> js-moderatorsWrapper">
      <!-- // placeholder so border collapse doesn't erase the table border when there are no table cells -->

      <div v-if="ob.placeholder" class="moderatorCard clrBr">
        <div class="moderatorCardInner">
        </div>
      </div>

      <div v-if="ob.totalIDs && !ob.totalShown" class="moderatorCard moderatorsMessage clrBr">
        <div class="moderatorCardInner">
          <div class="flexCent">
            <div class="flexColRows flexHCent gutterVTn">

              <h4>{{ ob.polyT(`${msgPath}.title`, opts) }}</h4>
              <!-- // The section below is only relevant if the moderators are loaded in a purchasing context. -->
              <div v-if="ob.purchase">
                <div class="tx4 clrT2">{{ ob.polyT(`${msgPath}.body`) }}</div>

                <div v-if="showOnlyVer">
                  <!-- //just a spacer -->
                  <div class="padTn"></div>
                  <div>
                    <button class="btn clrP clrBr js-showUnverified">{{ ob.polyT('moderators.showUnverified') }}</button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="js-statusWrapper"></div>

  </div>
</template>

<script setup>
const props = defineProps({
  feeLevel: String,
})

const showOnlyVer = ob.showVerifiedOnly && ob.unVerCount ? 'Verified' : '';
const showOnlyCur = ob.showOnlyCur ? 'Matching' : '';
const opts = ob.showOnlyCur ? { coin: ob.showOnlyCur } : {};
const msgPath = `moderators.noModsMsg.no${showOnlyCur}${showOnlyVer}Moderators`;
</script>
<style lang="scss" scoped>
</style>