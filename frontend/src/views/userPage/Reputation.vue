<template>
  <div class="userPageReputation">
    <div class="flexColRows gutterVLg">
      <div class="contentBox flexRow flexVCent gutterH pad clrP clrBr statsBox">
        <div v-if="!ob.isFetching">
          <div class="col6 txCtr">
            <div class="repBg">{{ ob.formatRating(ob.average, '', true) }}</div>
            <div class="tx2b">{{ ob.polyT('reputation.averageRating') }}</div>
          </div>
          <div class="rowDivV clrBrBk"></div>
          <div class="col6 txCtr">
            <div class="repBg">{{ ob.count }}</div>
            <div class="tx2b">{{ ob.polyT('reputation.totalReviews', { smart_count: ob.count }) }}</div>
          </div>
        </div>

        <div v-else>
          <div class="flexHCent">
            <SpinnerSVG className="spinnerMd" />
          </div>
        </div>
      </div>
      <div v-if="!ob.isFetching">
        <div class="js-reviews"></div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../backbone/app';
import Reviews from '../../../backbone/views/reviews/Reviews';
import { openSimpleMessage } from '../../../backbone/modals/SimpleMessage';
import Profile from '../../../backbone/models/profile/Profile';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.model || !(options.model instanceof Profile)) {
        throw new Error('Please provide a valid profile model.');
      }
      const opts = {
        ...options,
        initialState: {
          isFetching: true,
          ...options.initialState || {},
        },
      };
      this.setState(opts.initialState || {});
      this.options = opts;

      // create the reviews here, so they're available for the fetch
      this.reviews = this.createChild(Reviews, {
        async: true,
        initialPageSize: 5,
        pageSize: 5,
        initialState: {
          isFetchingRatings: true,
        },
      });

      // fetch the ratings immediately. They are asyncronous, and should not be refetched
      // if the view re-renders.
      this.ratingsFetch =
        $.get(app.getServerUrl(`ob/ratingindex/${this.options.model.get('peerID')}`))
          .done(data => this.onRatings(data))
          .fail((jqXhr) => {
            if (jqXhr.statusText === 'abort') return;
            const failReason = jqXhr.responseJSON && jqXhr.responseJSON.reason || '';
            openSimpleMessage(
              app.polyglot.t('listingDetail.errors.fetchRatings'),
              failReason);
          });
    },

    onRatings (data) {
      const pData = data || {};
      this.setState({
        isFetching: false,
        ...pData,
      });
      this.reviews.reviewIDs = pData.ratings || [];
      this.reviews.setState({ isFetchingRatings: false });
    },

    remove () {
      if (this.ratingsFetch) this.ratingsFetch.abort();
    },

    render () {
      this.delegateEvents(this.reviews);
      this.getCachedEl('.js-reviews').append(this.reviews.render().$el);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
