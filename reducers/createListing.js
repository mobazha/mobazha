import { createAction, handleActions } from 'redux-actions';
import { take, get } from 'lodash';
import {I18n} from '../langs/I18n';

export const actions = {
  editListing: 'CREATE_LISTING/EDIT_EXISTING_ONE',
  setEditListingData: 'CREATE_LISTING/SET_EXISTING_DATA',
  resetData: 'CREATE_LISTING/RESET',
  createListing: 'CREATE_LISTING/CREATE',
  actionSuccess: 'CREATE_LISTING/ACTION_SUCCESS',
  actionFailed: 'CREATE_LISTING/ACTION_FAILED',
  updateListing: 'CREATE_LISTING/UPDATE',
  deleteListing: 'CREATE_LISTING/DELETE',
  setImages: 'CREATE_LISTING/SET_IMAGES',
  setBasicInfo: 'CREATE_LISTING/SET_BASIC_INFO',
  setDetails: 'CREATE_LISTING/SET_DETAIL',
  updateOptions: 'CREATE_LISTING/UPDATE_OPTIONS',
  updateInventory: 'CREATE_LISTING/UPDATE_INVENTORY',
  setWorldWideShipping: 'CREATE_LISTING/SET_WORLD_WIDE_SHIPPING',
  setShippingOption: 'CREATE_LISTING/SET_SHIPPING',
  addShippingOption: 'CREATE_LISTING/ADD_SHIPPING',
  updateShippingOption: 'CREATE_LISTING/UPDATE_SHIPPING',
  removeShippingOption: 'CREATE_LISTING/REMOVE_SHIPPING',
  addCoupon: 'CREATE_LISTING/ADD_COUPON',
  editCoupon: 'CREATE_LISTING/EDIT_COUPON',
  removeCoupon: 'CREATE_LISTING/REMOVE_COUPON',
  trackInventory: 'CREATE_LISTING/TRACK_INVENTORY',
  resetInventory: 'CREATE_LISTING/RESET_INVENTORY',
};

export const resetData = createAction(actions.resetData);
export const createListing = createAction(actions.createListing);
export const updateListing = createAction(actions.updateListing);
export const setImages = createAction(actions.setImages);
export const setBasicInfo = createAction(actions.setBasicInfo);
export const setDetails = createAction(actions.setDetails);
export const updateOptions = createAction(actions.updateOptions);
export const updateInventory = createAction(actions.updateInventory);
export const setWorldWideShipping = createAction(actions.setWorldWideShipping);
export const setShippingOption = createAction(actions.setShippingOption);
export const addShippingOption = createAction(actions.addShippingOption);
export const removeShippingOption = createAction(actions.removeShippingOption);
export const updateShippingOption = createAction(actions.updateShippingOption);
export const editListing = createAction(actions.editListing);
export const addCoupon = createAction(actions.addCoupon);
export const editCoupon = createAction(actions.editCoupon);
export const removeCoupon = createAction(actions.removeCoupon);
export const trackInventory = createAction(actions.trackInventory);
export const resetInventory = createAction(actions.resetInventory);
export const deleteListing = createAction(actions.deleteListing);

const initialState = {
  step: 'creation',
  slug: '',
  images: [],
  nsfw: false,
  title: '',
  description: '',
  price: '0',
  type: '',
  tags: [],
  condition: '',
  categories: [],
  termsAndConditions: '',
  refundPolicy: '',
  options: [],
  inventory: [],
  coupons: [],
  shippingOptions: [],
  worldWideShipping: true,
  inventoryTracking: false,
  status: '',
};

const generateSKUs = (options, title = '') => {
  let result = [];
  if (options.length > 0) {
    result = options[0].variants.map((variant, idx) => ({
      productID: variant.toLowerCase(),
      variantCombo: [idx],
    }));

    for (let i = 1; i < options.length; i += 1) {
      let tmp = [];
      for (let j = 0; j < options[i].variants.length; j += 1) {
        const paritalResult = result.map(item => ({
          productID: `${item.productID}-${options[i].variants[j]}`,
          variantCombo: [...item.variantCombo, j],
          quantity: -1,
          surcharge: 0,
        }));
        tmp = [...tmp, ...paritalResult];
      }
      result = tmp;
    }
  } else {
    result = [
      {
        productID: title,
        variantCombo: [],
        quantity: -1,
        surcharge: 0,
      },
    ];
  }
  return result;
};

const generateInventory = (skus, value) => {
  const inventory = [];
  for (let i = 0; i < skus.length; i += 1) {
    inventory.push({
      ...skus[i],
      quantity: value,
      surcharge: 0,
    });
  }
  return inventory;
};

export default handleActions(
  {
    [actions.resetData]: state => ({
      ...state,
      step: 'creation',
      slug: '',
      images: [],
      nsfw: false,
      title: '',
      description: '',
      price: '0',
      type: 'physical_good',
      tags: [],
      condition: 'new',
      categories: [],
      termsAndConditions: '',
      refundPolicy: '',
      options: [],
      inventory: [{
        productId: '',
        quantity: -1,
        surcharge: 0,
        variantCombo: [],
      }],
      coupons: [],
      shippingOptions: [
        {
          name: I18n.t('reducers.createListing.Free_Worldwide_Shipping'),
          type: 'FIXED_PRICE',
          regions: ['ALL'],
          services: [
            {
              name: I18n.t('reducers.createListing.Standard'),
              price: 0,
              estimatedDelivery: I18n.t('reducers.createListing.days_30'),
              additionalItemPrice: 0,
            },
          ],
        },
      ],
      worldWideShipping: true,
    }),
    [actions.setImages]: (state, action) => ({
      ...state,
      images: action.payload,
    }),
    [actions.setBasicInfo]: (state, action) => ({
      ...state,
      ...action.payload,
    }),
    [actions.setDetails]: (state, action) => ({ ...state, ...action.payload }),
    [actions.updateOptions]: (state, action) => {
      const options = action.payload;
      const sku = generateSKUs(options);
      const inventory = state.inventoryTracking ? generateInventory(sku, 1) : generateInventory(sku, -1);
      return {
        ...state,
        options,
        inventory,
      };
    },
    [actions.updateInventory]: (state, action) => {
      const { inventory } = state;
      const { idx, value } = action.payload;
      const newInventory = take(inventory, inventory.length);
      newInventory[idx] = { ...newInventory[idx] };
      newInventory[idx].productId = get(value, 'productId');
      newInventory[idx].surcharge = get(value, 'surcharge', 0);
      newInventory[idx].quantity = get(value, 'quantity', -1);
      return {
        ...state,
        inventory: newInventory,
      };
    },
    [actions.setWorldWideShipping]: (state, action) => {
      if (action.payload === true) {
        return { ...state, worldWideShipping: action.payload };
      }
      return { ...state, worldWideShipping: action.payload, shippingOptions: [] };
    },
    [actions.setShippingOption]: (state, action) => ({ ...state, shippingOptions: action.payload }),
    [actions.addShippingOption]: (state, action) => {
      const shippingOptions = [...state.shippingOptions];
      shippingOptions.push(action.payload);
      return {
        ...state,
        shippingOptions,
      };
    },
    [actions.updateShippingOption]: (state, action) => {
      const shippingOptions = [...state.shippingOptions];
      const { idx, shippingOption } = action.payload;
      shippingOptions[idx] = shippingOption;
      return {
        ...state,
        shippingOptions,
      };
    },
    [actions.removeShippingOption]: (state, { payload }) => {
      const shippingOptions = [
        ...state.shippingOptions.slice(0, payload),
        ...state.shippingOptions.slice(payload + 1),
      ];
      return {
        ...state,
        shippingOptions,
      };
    },
    [actions.setEditListingData]: (state, action) => ({
      ...state,
      ...action.payload,
    }),
    [actions.addCoupon]: (state, { payload }) => {
      const coupons = [...state.coupons];
      coupons.push(payload);
      return {
        ...state,
        coupons,
      };
    },
    [actions.editCoupon]: (state, { payload: { selectedCoupon, value } }) => {
      const coupons = [...state.coupons];
      coupons[selectedCoupon] = value;
      return { ...state, coupons };
    },
    [actions.removeCoupon]: (state, { payload }) => {
      const coupons = [...state.coupons.slice(0, payload), ...state.coupons.slice(payload + 1)];
      return {
        ...state,
        coupons,
      };
    },
    [actions.resetInventory]: (state) => {
      const sku = generateSKUs(state.options);
      const inventory = generateInventory(sku, -1);
      return {
        ...state,
        inventory,
        inventoryTracking: false,
      };
    },
    [actions.trackInventory]: (state) => {
      const sku = generateSKUs(state.options);
      const inventory = generateInventory(sku, 1);
      return {
        ...state,
        inventory,
        inventoryTracking: true,
      };
    },
    [actions.createListing]: state => ({
      ...state,
      status: 'creating',
    }),
    [actions.actionSuccess]: state => ({
      ...state,
      status: 'success',
    }),
    [actions.actionFailed]: state => ({
      ...state,
      status: 'failed',
    }),
    [actions.updateListing]: state => ({
      ...state,
      status: 'updating',
    }),
    [actions.deleteListing]: state => ({
      ...state,
      status: 'deleting',
    }),
  },
  initialState,
);
