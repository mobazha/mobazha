<template>
  <div class="reviews">
    <div class="contentBox padLg clrP clrBr clrSh3">
      <h2 class="txUnb title">{{ ob.polyT('listingDetail.reviews') }}</h2>
      <template v-if="!options.isFetchingRatings">
        <template v-if="reviewIDs.length">
          <div ref="reviewWrapper" class="js-reviewWrapper" v-show="!!collection.length">
            <template v-for="review in collection" :key="review.cid">
              <Review :options="{ model: review, showListingData, }" />
            </template>
          </div>
          <div class="clrTErr js-errors" v-html="errorMsg"></div>
          <div class="flexHCent loadMore clrBr" v-show="!!collection.length && startIndex < reviewIDs.length">
            <ProcessingButton :className="`btn clrP clrBr js-loadMoreBtn ${loadingMore ? 'processing' : ''}`" @click="clickLoadMore"
              :btnText="ob.polyT('listingDetail.review.loadMore')" />
          </div>
          <div class="flexHCent js-reviewsSpinner" v-show="!collection.length">
            <SpinnerSVG className="spinnerMd" />
          </div>
        </template>

        <template v-else>
          <div class="noReviews">
            <i class="clrT2">{{ ob.polyT('listingDetail.review.noReviews') }}</i>
          </div>
        </template>
      </template>

      <template v-else>
        <div class="flexHCent">
          <SpinnerSVG className="spinnerMd" />
        </div>
      </template>
    </div>
  </div>
</template>

<script>
/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import { myAjax } from '../../api/api';
import { getSocket } from '../../../backbone/utils/serverConnect';
import app from '../../../backbone/app';
import Collection from '../../../backbone/collections/Reviews';

import Review from './Review.vue';

export default {
  components: {
    Review,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    reviewIDs: {
      type: Object,
      default: [],
    },
    bb: Function,
  },
  data () {
    return {
      _collection: new Collection(),
      _collectionKey: 0,

      startIndex: 0,
      initialPageSize: 3,
      pageSize: 10,
      showListingData: false,

      loadingMore: false,
      errorMsg: '',
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    if (this.options.async) {
      this.listenForReviews();
    }
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        reviewsLength: this.reviewIDs.length,
        ...this._state,
      };
    },

    collection() {
      let access = this._collectionKey;

      return this._collection;
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      this.startIndex = this.options.startIndex || 0;
      this.initialPageSize = this.options.pageSize || 3;
      this.pageSize = this.options.pageSize || 10;
      this.showListingData = this.options.showListingData;

      this._collection.on('change', () => this._collectionKey += 1);

      // load the reviews when data is available and the collection is empty
      if (this.reviewIDs.length && !this._collection.length) {
        this.loadReviews(this.startIndex, this.initialPageSize);
      }
    },

    onSocketMessage (event) {
      const eventData = event.jsonData || {};
      if (this.reviewIDs && this.reviewIDs.indexOf(eventData.ratingId) !== -1) {
        if (!eventData.error) {
          this._collection.add(eventData.rating.ratingData);
        } else {
          // add the error to the collection so it can be shown in place of the review
          this._collection.add(eventData);
        }
        if (this._collection.length >= this.startIndex) {
          this.loadingMore = false;
        }
      }
    },

    listenForReviews () {
      const serverSocket = getSocket();

      if (serverSocket) {
        this.stopListening(serverSocket, null, this.onSocketMessage);
        this.listenTo(serverSocket, 'message', this.onSocketMessage);
      } else {
        throw new Error('There is no connection to the server to listen to.');
      }
    },

    appendError (error) {
      const msg = app.polyglot.t('listingDetail.errors.fetchReviews', { error });
      this.errorMsg += `<p><i class="ion-alert-circled"> ${msg}</p>`;
    },

    loadReviews (start = this.startIndex, pageSize = this.pageSize, async = !!this.options.async) {
      const asyncUpdate = false;

      const revLength = this.reviewIDs.length;
      // if on the last page, only fetch the number of reviews that remain
      const ps = start + pageSize <= revLength ? pageSize : revLength - start;

      if (start < revLength) {
        this.loadingMore = true;
        this.errorMsg = '';
        myAjax({
          url: app.getServerUrl(`ob/fetchratings?async=${asyncUpdate}`),
          data: JSON.stringify(this.reviewIDs.slice(start, start + ps)),
          dataType: 'json',
          contentType: 'application/json',
          type: 'POST',
        })
          .done((data) => {
            this.startIndex = start + ps;
            if (!asyncUpdate) {
              this.collection.add(_.pluck(data, 'rating'));
              this.loadingMore = false;
            }
          })
          .fail((xhr) => {
            const failReason = (xhr.responseJSON && xhr.responseJSON.reason) || '';
            this.appendError(failReason);
            this.errorMsg += `<p>${failReason}</p>`;
            this.loadingMore = false;
          });
      }
    },

    clickLoadMore () {
      this.loadReviews(this.startIndex);
    },
  }
}
</script>
<style lang="scss" scoped></style>
