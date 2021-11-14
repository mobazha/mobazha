import { createAction, handleActions } from 'redux-actions';

export const actions = {
  fetchFeatured: 'PRODUCT/FETCH_FEATURED',
  setFeatured: 'PRODUCT/SET_FEATURED',
  setFeaturedLoading: 'PRODUCT/SET_FEATURED_LOADING',

  fetchTrending: 'PRODUCT/FETCH_TRENDING',
  setTrending: 'PRODUCT/SET_TRENDING',
  setTrendingLoading: 'PRODUCT_SET_TRENDING_LOADING',

  fetchBestsellers: 'PRODUCT/FETCH_BESTSELLERS',
  setBestsellers: 'PRODUCT/SET_BESTSELLERS',
  setBestsellersLoading: 'PRODUCT/SET_BESTSELLERS_LOADING',

  fetchGaming: 'PRODUCT/FETCH_GAMING',
  setGaming: 'PRODUCT/SET_GAMING',
  setGamingLoading: 'PRODUCT/SET_GAMING_LOADING',

  fetchMunchies: 'PRODUCT/FETCH_MUNCHIES',
  setMunchies: 'PRODUCT/SET_MUNCHIES',
  setMunchiesLoading: 'PRODUCT/SET_MUNCHIES_LOADING',

  fetchArts: 'PRODUCT/FETCH_ARTS',
  setArts: 'PRODUCT/SET_ARTS',
  setArtsLoading: 'PRODUCT/SET_ARTS_LOADING',

  fetchDevices: 'PRODUCT/FETCH_DEVICES',
  setDevices: 'PRODUCT/SET_DEVICES',
  setDevicesLoading: 'PRODUCT/SET_DEVICES_LOADING',

  fetchListingsForCategories: 'PRODUCT/FETCH_LISTINGS_FOR_CATEGORIES',
  fetchListingsForCategory: 'PRODUCT/FETCH_LISTINGS_FOR_CATEGORY',
  setListingsForCategory: 'PRODUCT/SET_LISTINGS_FOR_CATEGORY',
  setListingsForCategoryLoading: 'PRODUCT/SET_LISTINGS_FOR_CATEGORY_LOADING',
};

export const fetchFeaturedListing = createAction(actions.fetchFeatured);
export const fetchTrendingListing = createAction(actions.fetchTrending);
export const fetchBestsellersListing = createAction(actions.fetchBestsellers);
export const fetchGamingListing = createAction(actions.fetchGaming);
export const fetchMunchiesListing = createAction(actions.fetchMunchies);
export const fetchArtsListing = createAction(actions.fetchArts);
export const fetchDevicesListing = createAction(actions.fetchDevices);
export const fetchListingsForCategories = createAction(actions.fetchListingsForCategories);

const initialState = {
  featured: { data: [], loading: false },
  trending: { data: [], loading: false },
  bestsellers: { data: [], loading: false },
  gaming: { data: [], loading: false },
  munchies: { data: [], loading: false },
  devices: { data: [], loading: false },
  listingsForCategories: {},
};

export default handleActions({
  [actions.fetchFeatured]: state => ({
    ...state,
    featured: {
      loading: true,
      data: state.featured.data,
    },
  }),
  [actions.setFeatured]: (state, action) => ({
    ...state,
    featured: {
      loading: false,
      data: action.payload,
    },
  }),
  [actions.setFeaturedLoading]: (state, action) => ({
    ...state,
    featured: {
      loading: action.payload,
      data: state.featured.data,
    },
  }),

  [actions.fetchTrending]: state => ({
    ...state,
    trending: {
      loading: true,
      data: state.trending.data,
    },
  }),
  [actions.setTrending]: (state, action) => ({
    ...state,
    trending: {
      loading: false,
      data: action.payload,
    },
  }),
  [actions.setTrendingLoading]: (state, action) => ({
    ...state,
    featured: {
      loading: action.payload,
      data: state.trending.data,
    },
  }),

  [actions.fetchBestsellers]: state => ({
    ...state,
    bestsellers: {
      loading: true,
      data: state.bestsellers.data,
    },
  }),
  [actions.setBestsellers]: (state, action) => ({
    ...state,
    bestsellers: {
      loading: false,
      data: action.payload,
    },
  }),
  [actions.setBestsellersLoading]: (state, action) => ({
    ...state,
    featured: {
      loading: action.payload,
      data: state.bestsellers.data,
    },
  }),

  [actions.fetchGaming]: state => ({
    ...state,
    gaming: {
      loading: true,
      data: state.gaming.data,
    },
  }),
  [actions.setGaming]: (state, action) => ({
    ...state,
    gaming: {
      loading: false,
      data: action.payload,
    },
  }),
  [actions.setGamingLoading]: (state, action) => ({
    ...state,
    gaming: {
      loading: action.payload,
      data: state.gaming.data,
    },
  }),

  [actions.fetchMunchies]: state => ({
    ...state,
    munchies: {
      loading: true,
      data: state.munchies.data,
    },
  }),
  [actions.setMunchies]: (state, action) => ({
    ...state,
    munchies: {
      loading: false,
      data: action.payload,
    },
  }),
  [actions.setMunchiesLoading]: (state, action) => ({
    ...state,
    munchies: {
      loading: action.payload,
      data: state.munchies.data,
    },
  }),

  [actions.fetchDevices]: state => ({
    ...state,
    devices: {
      loading: true,
      data: state.devices.data,
    },
  }),
  [actions.setDevices]: (state, action) => ({
    ...state,
    devices: {
      loading: false,
      data: action.payload,
    },
  }),
  [actions.setDevicesLoading]: (state, action) => ({
    ...state,
    devices: {
      loading: action.payload,
      data: state.devices.data,
    },
  }),

  [actions.setListingsForCategory]: (state, action) => {
    const { name, results } = action.payload;
    return {
      ...state,
      listingsForCategories: {
        ...state.listingsForCategories,
        [name]: {
          loading: false,
          data: results,
        },
      },
    };
  },
  [actions.setListingsForCategoryLoading]: (state, action) => {
    const { name, loading } = action.payload;
    return {
      ...state,
      listingsForCategories: {
        ...state.listingsForCategories,
        [name]: {
          ...state.listingsForCategories[name],
          loading,
        },
      },
    };
  },
}, initialState);
