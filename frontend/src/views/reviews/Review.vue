<template>
  <div class="review clrBr">
    <template v-if="ob.error">
      <p class="clrTErr txCtr"><i class="ion-alert-circled"></i> {{ ob.polyT('listingDetail.errors.loadReview', { id: ob.ratingId, error: ob.error }) }}</p>
    </template>

    <template v-else>
      <div class="flexRow gutterHLg">
        <div class="col8 flex gutterHMd">
          <template v-if="!ob.showListingData">
            <template v-if="slugLink">
              <a :href="slugLink"><img class="thumbHg flexNoShrink" :src="ob.vendorSig.thumbnail ? background : '~@/../imgs/defaultItem.png'" /></a>
            </template>

            <template v-else>
              <img class="thumbHg flexNoShrink" :src="ob.vendorSig.thumbnail ? background : '~@/../imgs/defaultItem.png'" />
            </template>
          </template>
          <div class="flexExpand gutterVSm">
            <div class="tx5b clrT2">
              <template v-if="ob.buyerID">
                <b>
                  <div v-html='ob.polyT("listingDetail.review.title", {
                    time: ob.moment(ob.timestamp).format("MMM Do YYYY h:mm a"),
                    name: `<a href="${ob.buyerID.peerID}"><span class="clrT2">${ob.buyerName}</span></a>`
                    })'></div>
                </b>
              </template>

              <template v-else>
                <b>{{ ob.moment(ob.timestamp).format('MMM Do YYYY h:mm a') }}</b>
              </template>
            </div>
            <template v-if="!ob.showListingData">
              <h4 class="clrT">
                <template v-if="slugLink">
                  <a :href="slugLink" class="clrT">{{ title }}</a>
                </template>

                <template v-else>
                  {{ title }}
                </template>
              </h4>
            </template>
            <div class="reviewTextWrapper js-reviewTextWrapper">
              <TextClamp :text="ob.review ?? ''" class="reviewText" autoresize :max-lines="model.get('buyerID') !== undefined ? 5 : 6" ellipsis="..." location="end" >
                <template #after="{ toggle, expanded, clamped }">
                  <button v-if="expanded || clamped" class="btnTxtOnly trunkLink" @click="toggle">
                  {{ clamped ? ob.polyT('listingDetail.review.showMore') : ob.polyT('listingDetail.review.showLess')}}</button>
                </template>
              </TextClamp>
            </div>
          </div>
        </div>
        <div class="col4">
          <table class="ratings">
            <tr>
              <td><b>{{ ob.polyT('ratingLabels.overall') }}</b></td>
              <td class="ratingsContainer">
                <RatingsStrip :rating="model.get('overall')" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.quality') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :rating="model.get('quality')" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.asAdvertised') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :rating="model.get('description')" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.delivery') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :rating="model.get('deliverySpeed')" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.service') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :rating="model.get('customerService')" />
              </td>
            </tr>
          </table>
        </div>
      </div>
    </template>

  </div>
</template>

<script>
import TextClamp from 'vue3-text-clamp';
import moment from 'moment';

import RatingsStrip from '../RatingsStrip.vue';

export default {
  components: {
    RatingsStrip,
    TextClamp,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
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
        moment,
        showListingData: this.options.showListingData,
        ...this.model.toJSON(),
      };
    },
    title () {
      const ob = this.ob;

      let niceSlug = ob.vendorSig.slug.replace(/-/g, ' ');
      niceSlug = niceSlug.replace(/\b\w/g, m => m.toUpperCase());
      let title = ob.vendorSig.title ? ob.vendorSig.title : niceSlug;
      return title ? title : ob.polyT('reputation.noTitle');
    },
    slugLink () {
      const ob = this.ob;
      return ob.vendorID && ob.vendorSig.slug ? `ob://${ob.vendorID.peerID}/store/${ob.vendorSig.slug}` : '';
    },
    background () {
      return ob.getServerUrl(`ob/image/${ob.isHiRez() ? ob.vendorSig.metadata.thumbnail.small : ob.vendorSig.metadata.thumbnail.tiny}`);
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);
    },
  }
}
</script>
<style lang="scss" scoped></style>
