import { Alert } from 'react-native';
import { call, put, takeEvery, select } from 'redux-saga/effects';
import { get, hasIn, find } from 'lodash';

import { actions } from '../createListing';
import { actions as listingAction } from '../listings';
import { actions as storeListingAction } from '../storeListings';

import { getUserCredentails, getNewListingInfo, getWalletList, getModerators } from './sagaSelector';
import { convertorsMap } from '../../selectors/currency';
import { restructureListingInfo, parseCoupon } from '../../utils/dataMapping';
import { createListing, updateListing, deleteListing } from '../../api/products';
import { eventTracker } from '../../utils/EventTracker';

import {I18n} from '../../langs/I18n';

const parseOptions = (options) => {
  const parsedData = options.map((val) => {
    const { name, description, variants } = val;
    return {
      name,
      description,
      variants: variants.map(val => val.name),
    };
  });
  return parsedData;
};

const parseSkus = (skus, convertor, pricingCurrency) => {
  const parsedData = skus.map((val) => {
    const {
      productID, bigQuantity: quantity, bigSurcharge: surcharge, variantCombo,
    } = val;
    return {
      productId: productID,
      quantity,
      surcharge: convertor.convertCurrency(surcharge, pricingCurrency),
      variantCombo: variantCombo ? [...variantCombo] : [],
    };
  });
  return parsedData;
};

const parseShippingOptions = (options = [], convertor, pricingCurrency) => {
  const parsedData = options.map((option) => {
    const services = option.services || [];
    const regions = option.regions || [];
    const parsedRegions = regions.map(region => region.toLowerCase());
    const parsedServices = services.map(service => ({
      ...service,
      additionalItemPrice: convertor.convertCurrency(service.bigAdditionalItemPrice, pricingCurrency),
      price: convertor.convertCurrency(service.bigPrice, pricingCurrency),
    }));
    return ({
      ...option,
      regions: parsedRegions,
      services: parsedServices,
    });
  });
  return parsedData;
};

const parseImages = (images) => {
  const parsedData = images.map((val) => {
    const {
      filename, tiny, small, medium, large, original,
    } = val;
    return {
      filename,
      hashes: {
        tiny,
        small,
        medium,
        large,
        original,
      },
    };
  });
  return parsedData;
};

function* generateData (isCreating = true) {
  const listingInfo = yield select(getNewListingInfo);
  const acceptedCurrencies = yield select(getWalletList);
  const moderators = yield select(getModerators);
  return restructureListingInfo(listingInfo, acceptedCurrencies, moderators, isCreating);
}

function* createListingAction(action) {
  const successAction = action.payload;
  const { username, password } = yield select(getUserCredentails);
  const structuredData = yield call(generateData);
  const result = yield call(createListing, username, password, structuredData);
  if (hasIn(result, 'slug')) {
    yield put({ type: actions.resetData });
    yield put({ type: actions.actionSuccess });
    yield call(successAction, get(result, 'slug'));
    eventTracker.trackEvent('CreateListing-Created');
  } else {
    eventTracker.trackEvent('CreateListing-FailedCreation', result.reason);
    const metaType = result.reason.split('Listing_Metadata_')[1];
    yield put({ type: actions.actionFailed });
    if (metaType) {
      Alert.alert(`${metaType} is missing`);
    } else {
      Alert.alert(result.reason);
    }
  }
}

function* updateListingAction(action) {
  const { slug, peerID } = action.payload;
  const { username, password } = yield select(getUserCredentails);
  const structuredData = yield call(generateData, false)
  const result = yield call(updateListing, username, password, structuredData);
  if (hasIn(result, 'success') && result.success === false) {
    const metaType = result.reason.split('Listing_Metadata_')[1];
    yield put({ type: actions.actionFailed });
    if (metaType) {
      Alert.alert(`${metaType} is missing`);
    } else {
      Alert.alert(result.reason);
    }
  } else {
    yield put({
      type: listingAction.fetchListing,
      payload: { slug, peerID },
    });
    yield put({ type: storeListingAction.fetchListings });
    yield put({ type: actions.actionSuccess });
  }
}

function* deleteListingAction(action) {
  const { slug } = action.payload;
  const { username, password } = yield select(getUserCredentails);
  yield call(deleteListing, username, password, slug);
  yield put({
    type: storeListingAction.fetchListings,
  });
}

function* editListingAction(action) {
  const {
    listingInfo: {
      listing: {
        slug,
        item: {
          images,
          nsfw,
          title,
          description,
          bigPrice,
          tags,
          condition,
          categories,
          options,
          skus,
          priceCurrency: { code },
        },
        metadata: { contractType },
        termsAndConditions,
        refundPolicy,
        shippingOptions,
        coupons,
      },
    },
  } = action.payload;
  const convertor = yield select(convertorsMap);
  const data = {
    stage: 'edit',
    slug,
    images: parseImages(images),
    nsfw,
    title,
    description,
    tags,
    coupons: (coupons || []).map(parseCoupon),
    price: convertor.convertCurrency(bigPrice, code),
    condition: condition.toLowerCase(),
    categories,
    options: parseOptions(options),
    inventory: parseSkus(skus, convertor, code),
    type: contractType.toLowerCase(),
    termsAndConditions,
    refundPolicy: refundPolicy,
    shippingOptions: parseShippingOptions(shippingOptions, convertor, code),
    worldWideShipping: false,
  };
  yield put({ type: actions.setEditListingData, payload: data });
}

const CreateListingSaga = function* Search() {
  yield takeEvery(actions.createListing, createListingAction);
  yield takeEvery(actions.editListing, editListingAction);
  yield takeEvery(actions.updateListing, updateListingAction);
  yield takeEvery(actions.deleteListing, deleteListingAction);
};

export default CreateListingSaga;
