<template>
  <div class="reviews">
    <div class="contentBox padLg clrP clrBr clrSh3">
      <h2 class="txUnb title">{{ ob.polyT('listingDetail.reviews') }}</h2>
      <template v-if="!ob.isFetchingRatings">
        <template v-if="ob.reviewsLength">
          <div ref="reviewWrapper" class="js-reviewWrapper" v-show="!!ob.collectionLength"></div>
          <div class="clrTErr js-errors" v-html="errorMsg"></div>
          <div class="flexHCent loadMore clrBr" v-show="!!ob.collectionLength && startIndex < reviewIDs.length">
            <ProcessingButton :className="`btn clrP clrBr js-loadMoreBtn ${loadingMore ? 'processing' : ''}`" @click="clickLoadMore"
              :btnText="ob.polyT('listingDetail.review.loadMore')" />
          </div>
          <div class="flexHCent js-reviewsSpinner" v-show="!ob.collectionLength">
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
import $ from 'jquery';
import _ from 'underscore';
import loadTemplate from '../../../backbone/utils/loadTemplate';
import { getSocket } from '../../../backbone/utils/serverConnect';
import app from '../../../backbone/app';
import Collection from '../../../backbone/collections/Reviews';
import Review from './Review';
import 'trunk8';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      reviewIDs: [],
      collection: [],

      loadingMore: false,
      errorMsg: '',
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();

    if (this.options.async) {
      this.listenForReviews();
    }
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        reviewsLength: this.reviewIDs.length,
        collectionLength: this.collection.length,
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        ...options,
        initialState: {
          isFetchingRatings: false, // pass in true if ratings are provided after the first render
          ...options.initialState || {},
        },
      };
      this.baseInit(opts);

      this.startIndex = this.options.startIndex || 0;
      this.initialPageSize = this.options.pageSize || 3;
      this.pageSize = this.options.pageSize || 10;
      this.reviewIDs = this.options.ratings || [];
      this.showListingData = this.options.showListingData;
      this.collection = new Collection();
      this.listenTo(this.collection, 'add', (model) => this.addReview(model));
    },

    onSocketMessage (event) {
      const eventData = event.jsonData || {};
      if (this.reviewIDs && this.reviewIDs.indexOf(eventData.ratingId) !== -1) {
        if (!eventData.error) {
          this.collection.add(eventData.rating.ratingData);
        } else {
          // add the error to the collection so it can be shown in place of the review
          this.collection.add(eventData);
        }
        if (this.collection.length >= this.startIndex) {
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
        $.ajax({
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

    addReview (model) {
      const newReview = new Review({
        model,
        showListingData: this.showListingData,
      });
      const newRevieEl = newReview.render().$el;
      const btnTxt = app.polyglot.t('listingDetail.review.showMore');
      const truncLines = model.get('buyerID') !== undefined ? 5 : 6;

      $(this.$refs.reviewWrapper).append(newRevieEl);

      // truncate any review text that is too long
      newRevieEl.find('.js-reviewText').trunk8({
        fill: `… <button class="btnTxtOnly trunkLink js-showMore">${btnTxt}</button>`,
        lines: truncLines,
      });
    },

    clickLoadMore () {
      this.loadReviews(this.startIndex);
    },

    render () {
      super.render();
      loadTemplate('reviews/reviews.html', (t) => {
        this.$el.html(t({
          reviewsLength: this.reviewIDs.length,
          collectionLength: this.collection.length,
          ...this.getState(),
        }));

        // render any reviews that have already loaded
        this.collection.each((review) => this.addReview(review));

        // load the reviews when data is available and the collection is empty
        if (this.reviewIDs.length && !this.collection.length) {
          this.loadReviews(this.startIndex, this.initialPageSize);
        }
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
