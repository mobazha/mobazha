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
              <a class="thumbHg flexNoShrink" :style="`background-image: url(${background}), url('../imgs/defaultItem.png')`" :href="slugLink">
              </a>
            </template>

            <template v-else>
              <div class="thumbHg flexNoShrink" :style="`background-image: url(${background}), url('../imgs/defaultItem.png')`">
              </div>
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
              <p class="reviewText js-reviewText">{{ ob.review }}</p>
            </div>
          </div>
        </div>
        <div class="col4">
          <table class="ratings">
            <tr>
              <td><b>{{ ob.polyT('ratingLabels.overall') }}</b></td>
              <td class="ratingsContainer">
                <RatingsStrip :options="{ curRating: model.get('overall') || 0, }" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.quality') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :options="{ curRating: model.get('quality') || 0, }" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.asAdvertised') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :options="{ curRating: model.get('description') || 0, }" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.delivery') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :options="{ curRating: model.get('deliverySpeed') || 0, }" />
              </td>
            </tr>
            <tr>
              <td>{{ ob.polyT('ratingLabels.service') }}</td>
              <td class="ratingsContainer">
                <RatingsStrip :options="{ curRating: model.get('customerService') || 0, }" />
              </td>
            </tr>
          </table>
        </div>
      </div>
    </template>

  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../backbone/app';
import moment from 'moment';
import 'trunk8';

import RatingsStrip from '../RatingsStrip.vue';


export default {
  components: {
    RatingsStrip,
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
      let background = '';
      if (ob.vendorSig.thumbnail) {
        background = ob.getServerUrl(`ob/image/${ob.isHiRez() ? ob.vendorSig.metadata.thumbnail.small : ob.vendorSig.metadata.thumbnail.tiny}`);
      }
      return background;
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);
    },

    events () {
      return {
        'click .js-showMore': 'clickShowMore',
        'click .js-showLess': 'clickShowLess',
      };
    },

    clickShowMore (e) {
      // the show more button is added by the parent view when it applies trunk8 to the text
      const btnTxt = app.polyglot.t('listingDetail.review.showLess');
      $(e.target).parent().trunk8('revert')
        .append(`&nbsp; <button class="btnTxtOnly trunkLink js-showLess">${btnTxt}</button>`);
    },

    clickShowLess (e) {
      $(e.target).parent().trunk8();
    },
  }
}
</script>
<style lang="scss" scoped></style>
