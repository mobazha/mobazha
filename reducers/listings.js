import { createAction, handleActions } from 'redux-actions';
import * as _ from 'lodash';

import {I18n} from '../langs/I18n';

const MAXIMUM_LISTING_COUNT = 100;

export const actions = {
  addListing: 'LISTING/ADD_LISTING',
  fetchListing: 'LISTING/FETCH_LISTING',
  loadingFromGatewayNow: 'LISTING/FETCH_LISTING_FROM_GATEWAY',
};

export const addListing = createAction(actions.addListing);
export const fetchListing = createAction(actions.fetchListing);

const initialState = {
  data: [],
  status: '',
};

export default handleActions(
  {
    [actions.fetchListing]: state => ({ ...state, status: 'LOADING_LISTING' }),
    [actions.loadingFromGatewayNow]: state => ({ ...state, status: 'LOADING_LISTING_FROM_GATEWAY' }),
    [actions.addListing]: (state, action) => {
      const slug = _.get(action, 'payload.listing.slug');
      if (slug) {
        const listings = state.data.filter(item => item.listing.slug !== slug);
        if (listings.length >= MAXIMUM_LISTING_COUNT) {
          listings.splice(0, 1);
        }
        listings.push(action.payload);
        return { data: listings, status: 'LISTING_LOADING_SUCCESS' };
      } else {
        return { ...state, status: 'LISTING_LOADING_FAILED' };
      }
    },
  },
  initialState,
);
