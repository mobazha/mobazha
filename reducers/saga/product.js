import { call, put, takeEvery } from 'redux-saga/effects';
import * as _ from 'lodash';

import { actions } from '../product';

import {
  fetchTrendingListing,
  fetchFeaturedListing,
  fetchBestsellersListing,
  fetchGamingListing,
  fetchMunchiesListing,
  fetchArtsListing,
  fetchDevicesListing,
} from '../../api/products';
import { shuffle, PREVIEWING_CATEGORIES } from '../../utils/listings';
import { getRandomSearch } from '../../api/search';

export function* getTrending() {
  try {
    const result = yield call(fetchTrendingListing);
    yield put({
      type: actions.setTrending,
      payload: result.results.results.map(item => item.data),
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setTrendingLoading,
      payload: false,
    });
  }
}

export function* getFeatured() {
  try {
    const result = yield call(fetchFeaturedListing);
    yield put({
      type: actions.setFeatured,
      payload: result.results.results.map(item => item.data),
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setFeaturedLoading,
      payload: false,
    });
  }
}

export function* getBestsellers() {
  try {
    const response = yield call(fetchBestsellersListing);
    const result = response.results.results.map(item => item.data);
    yield put({
      type: actions.setBestsellers,
      payload: shuffle(result),
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setBestsellersLoading,
      payload: false,
    });
  }
}

export function* getGaming() {
  try {
    const response = yield call(fetchGamingListing);
    const result = response.results.results.map(item => item.data);
    yield put({
      type: actions.setGaming,
      payload: shuffle(result),
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setGamingLoading,
      payload: false,
    });
  }
}

export function* getMunchies() {
  try {
    const response = yield call(fetchMunchiesListing);
    const result = response.results.results.map(item => item.data);
    yield put({
      type: actions.setMunchies,
      payload: shuffle(result),
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setMunchiesLoading,
      payload: false,
    });
  }
}

export function* getArts() {
  try {
    const response = yield call(fetchArtsListing);
    const result = response.results.results.map(item => item.data);
    yield put({
      type: actions.setArts,
      payload: shuffle(result),
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setArtsLoading,
      payload: false,
    });
  }
}

export function* getDevices() {
  try {
    const response = yield call(fetchDevicesListing);
    const result = response.results.results.map(item => item.data);
    yield put({
      type: actions.setDevices,
      payload: shuffle(result),
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setDevicesLoading,
      payload: false,
    });
  }
}

export function* getListingsForCategories() {
  for (let i = 0; i < PREVIEWING_CATEGORIES.length; i += 1) {
    const name = PREVIEWING_CATEGORIES[i].categoryName;
    yield put({
      type: actions.fetchListingsForCategory,
      payload: name,
    });
  }
}

export function* getListingsForCategory(action) {
  const name = action.payload;
  yield put({
    type: actions.setListingsForCategoryLoading,
    payload: { name, loading: true },
  });

  try {
    const response = yield call(getRandomSearch, name, '', 0, 8, '');
    yield put({
      type: actions.setListingsForCategory,
      payload: {
        name,
        results: _.get(response, 'results.results', []),
      },
    });
  } catch (err) {
    console.log('Product Saga Error: ', err);
    yield put({
      type: actions.setListingsForCategoryLoading,
      payload: { name, loading: false },
    });
  }
}

const ProductSaga = function* Product() {
  yield takeEvery(actions.fetchTrending, getTrending);
  yield takeEvery(actions.fetchFeatured, getFeatured);
  yield takeEvery(actions.fetchBestsellers, getBestsellers);
  yield takeEvery(actions.fetchGaming, getGaming);
  yield takeEvery(actions.fetchMunchies, getMunchies);
  yield takeEvery(actions.fetchDevices, getDevices);
  yield takeEvery(actions.fetchListingsForCategories, getListingsForCategories);
  yield takeEvery(actions.fetchListingsForCategory, getListingsForCategory);
};

export default ProductSaga;
