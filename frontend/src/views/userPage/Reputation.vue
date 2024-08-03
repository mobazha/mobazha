<template>
  <div class="userPageReputation">
    <div class="flexColRows gutterVLg">
      <div class="contentBox flexRow flexVCent gutterH pad clrP clrBr statsBox">
        <template v-if="!ob.isFetching">
          <div class="col6 txCtr">
            <div class="repBg" v-html="ob.formatRating(ob.average, '', true)"></div>
            <div class="tx2b">{{ ob.polyT('reputation.averageRating') }}</div>
          </div>
          <div class="rowDivV clrBrBk"></div>
          <div class="col6 txCtr">
            <div class="repBg">{{ ob.count }}</div>
            <div class="tx2b">{{ ob.polyT('reputation.totalReviews', { smart_count: ob.count }) }}</div>
          </div>
        </template>

        <div v-else class="flexHCent">
          <SpinnerSVG className="spinnerMd" />
        </div>
      </div>
      <template v-if="!ob.isFetching">
        <div ref="reviewsList" class="js-reviewsList">
          <Reviews ref="reviews" :key="reviewIDs" :reviewIDs="reviewIDs" :options="{
            async: true,
            initialPageSize: 5,
            pageSize: 5,
            isFetchingRatings: ob.isFetching,
          }"/>
        </div>
      </template>
    </div>
  </div>
</template>

<script>
import { myGet } from '../..//api/api';
import app from '../../../backbone/app';
import { openSimpleMessage } from '../../../backbone/views/modals/SimpleMessage';
import Profile from '../../../backbone/models/profile/Profile';

import Reviews from '../reviews/Reviews.vue';

export default {
  components: {
    Reviews,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      reviewIDs: [],

      _state: {
        isFetching: true,
      }
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
  },
  unmounted() {
    if (this.ratingsFetch) this.ratingsFetch.abort();
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData(options = {}) {
      if (!this.model || !(this.model instanceof Profile)) {
        throw new Error('Please provide a valid profile model.');
      }
      const opts = {
        ...options,
        initialState: {
          isFetching: true,
          ...(options.initialState || {}),
        },
      };
      this.baseInit(opts);

      // fetch the ratings immediately. They are asyncronous, and should not be refetched
      // if the view re-renders.
      this.ratingsFetch = myGet(app.getServerUrl(`ob/ratingindex/${this.model.get('peerID')}`))
        .done((data) => this.onRatings(data))
        .fail((jqXhr) => {
          if (jqXhr.statusText === 'abort') return;
          const failReason = (jqXhr.responseJSON && jqXhr.responseJSON.reason) || '';
          openSimpleMessage(app.polyglot.t('listingDetail.errors.fetchRatings'), failReason);
        });
    },

    onRatings(data) {
      const pData = data || {};
      this.setState({
        isFetching: false,
        ...pData,
      });

      this.reviewIDs = pData.ratings || [];
    },
  },
};
</script>
<style lang="scss" scoped></style>
