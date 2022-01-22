console.log('overshowing save verification modal on crypto listing');

import _ from 'underscore';
import is from 'is_js';
import bigNumber from 'bignumber.js';
import app from '../../app';
import { getCurrencyByCode as getCryptoCurrencyByCode } from '../../data/walletCurrencies';
import { getIndexedCountries } from '../../data/countries';
import { events as listingEvents, shipsFreeToMe } from './';
import {
  decimalToInteger,
  integerToDecimal,
  decimalToCurDef,
  isValidCoinDivisibility,
  getCoinDivisibility,
  CUR_VAL_RANGE_TYPES,
  defaultCryptoCoinDivisibility,
  UnrecognizedCurrencyError,
} from '../../utils/currency';
import { upToFixed } from '../../utils/number';
import BaseModel, { flattenAttrs } from '../BaseModel';
import Item from './Item';
import Metadata from './Metadata';
import ShippingOptions from '../../collections/listing/ShippingOptions.js';
import Coupons from '../../collections/listing/Coupons.js';

export default class extends BaseModel {
  constructor(attrs, options = {}) {
    super(attrs, options);
    this.guid = options.guid;
  }

  url() {
    // url is handled by sync, but backbone bombs if I don't have
    // something explicitly set
    return 'use-sync';
  }

  static getIpnsUrl(guid, slug) {
    if (typeof guid !== 'string' || !guid) {
      throw new Error('Please provide a guid as a non-empty ' +
        'string.');
    }

    if (typeof slug !== 'string' || !slug) {
      throw new Error('Please provide a slug as a non-empty ' +
        'string.');
    }

    return app.getServerUrl(`ob/listing/${guid}/${slug}`);
  }

  getIpnsUrl() {
    const slug = this.get('slug');

    if (!slug) {
      throw new Error('In order to fetch a listing via IPNS, a slug must be '
        + 'set as a model attribute.');
    }

    return this.constructor.getIpnsUrl(this.guid, slug);
  }

  static getIpfsUrl(cid) {
    if (typeof cid !== 'string' || !cid) {
      throw new Error('Please provide a cid as a non-empty ' +
        'string.');
    }

    return app.getServerUrl(`ob/listing/${cid}`);
  }

  getIpfsUrl(cid) {
    return this.constructor.getIpfsUrl(cid);
  }

  defaults() {
    return {
      termsAndConditions: '',
      refundPolicy: '',
      item: new Item(),
      metadata: new Metadata(),
      shippingOptions: new ShippingOptions(),
      coupons: new Coupons(),
    };
  }

  isNew() {
    return !this.get('slug');
  }

  get nested() {
    return {
      item: Item,
      metadata: Metadata,
      shippingOptions: ShippingOptions,
      coupons: Coupons,
    };
  }

  get shipsFreeToMe() {
    return shipsFreeToMe(this);
  }

  get max() {
    return {
      refundPolicyLength: 10000,
      termsAndConditionsLength: 10000,
      couponCount: 30,
      minPriceModifier: -99.99,
      maxPriceModifier: 1000,
    };
  }

  get isOwnListing() {
    if (this.guid === undefined) {
      throw new Error('Unable to determine ownListing ' +
        'because a guid has not been set on this model.');
    }

    return app.profile.id === this.guid;
  }

  get isCrypto() {
    return this.get('metadata')
      .get('contractType') === 'CRYPTOCURRENCY';
  }

  get price() {
    const item = this.get('item');

    if (this.isCrypto) {
      let modifier = 0;

      try {
        modifier = item.get('cryptoListingPriceModifier') || 0;
      } catch (e) {
        // pass
      }

      return {
        amount: bigNumber(1 + (modifier / 100)),
        currencyCode: item.get('cryptoListingCurrencyCode'),
        modifier,
      };
    }

    let amount = bigNumber();
    try {
      amount = item.get('price');
    } catch (e) {
      // pass
    }

    const metadata = this.get('metadata');
    let currencyCode = '';
    try {
      currencyCode = metadata.get('pricingCurrency').code;
    } catch (e) {
      // pass
    }

    return {
      amount,
      currencyCode,
    };
  }

  set(key, val, options = {}) {
    // Handle both `"key", value` and `{key: value}` -style arguments.
    let attrs;
    let opts = options;

    if (typeof key === 'object') {
      attrs = key;
      opts = val || {};
    } else {
      (attrs = {})[key] = val;
    }

    let contractType;

    try {
      contractType =
        this.get('metadata')
          .get('contractType');
    } catch (e) {
      // pass
    }

    try {
      contractType = attrs.metadata.contractType;
    } catch (e) {
      // pass
    }

    if (contractType !== 'CRYPTOCURRENCY') {
      let curCode;

      try {
        curCode =
          attrs.metadata
            .pricingCurrency
            .code;
      } catch (e) {
        // pass
      }

      if (
        typeof curCode === 'string' &&
        curCode
      ) {
        try {
          attrs.metadata = {
            ...attrs.metadata,
            pricingCurrency: {
              code: curCode,
              divisibility: getCoinDivisibility(curCode),
            },
          };
        } catch (e) {
          if (
            attrs.metadata &&
            typeof attrs.metadata.pricingCurrency === 'object'
          ) {
            delete attrs.metadata.pricingCurrency.divisibility;
            // validate will fail validation on the model in this scenario -
            // it's almost certainly a dev error
          }
        }
      }
    } else {
      if (attrs.meta.contractType === 'CRYPTOCURRENCY' &&
        typeof attrs.item.cryptoListingPriceModifier === 'number') {
        // round to two decimal places
        attrs.item.cryptoListingPriceModifier = parseFloat(upToFixed(attrs.item.cryptoListingPriceModifier, 2));
      }
    }

    return super.set(attrs, opts);
  }

  /**
   * Returns a new instance of the listing with mostly identical attributes. Certain
   * attributes like slug and hash will be stripped since they are not appropriate
   * if this listing is being used as a template for a new listing. This differs from
   * clone() which will maintain identical attributes.
   */
  cloneListing() {
    const clone = this.clone();
    clone.unset('slug');
    clone.unset('cid');
    clone.guid = this.guid;
    clone.lastSyncedAttrs = {};
    return clone;
  }

  validate(attributes) {
    let errObj = {};
    const addError = (fieldName, error) => {
      errObj[fieldName] = errObj[fieldName] || [];
      errObj[fieldName].push(error);
    };

    const attrs = {
      ...this.toJSON(),
      ...flattenAttrs(attributes),
    };

    const metadata = attrs.metadata;
    const contractType = metadata.contractType;
    const item = attrs.item;

    const curDefCurrency = {
      code: () => metadata.pricingCurrency.code,
      divisibility: () => metadata.pricingCurrency.divisibility,
    };

    if (!(attributes.item instanceof Item)) {
      addError('item', 'A nested Item model is required.');
    }

    if (!(attributes.metadata instanceof Metadata)) {
      addError('metadata', 'A nested Metadata model is required.');
    }

    if (!(attributes.shippingOptions instanceof ShippingOptions)) {
      addError('shippingOptions', 'A nested ShippingOptions collection is required.');
    }

    if (!(attributes.coupons instanceof Coupons)) {
      addError('coupons', 'A nested Coupons collection is required.');
    }

    if (attrs.refundPolicy) {
      if (is.not.string(attrs.refundPolicy)) {
        addError('refundPolicy', 'The return policy must be of type string.');
      } else if (attrs.refundPolicy.length > this.max.refundPolicyLength) {
        addError('refundPolicy', app.polyglot.t('listingModelErrors.returnPolicyTooLong'));
      }
    }

    if (attrs.termsAndConditions) {
      if (is.not.string(attrs.termsAndConditions)) {
        addError('termsAndConditions', 'The terms and conditions must be of type string.');
      } else if (attrs.termsAndConditions.length > this.max.termsAndConditionsLength) {
        addError('termsAndConditions',
          app.polyglot.t('listingModelErrors.termsAndConditionsTooLong'));
      }
    }

    if (contractType === 'PHYSICAL_GOOD') {
      if (!attrs.shippingOptions.length) {
        addError('shippingOptions', app.polyglot.t('listingModelErrors.provideShippingOption'));
      }
    }

    if (contractType === 'CRYPTOCURRENCY') {
      if (!item || !item.cryptoListingCurrencyCode || typeof item.cryptoListingCurrencyCode !== 'string') {
        addError('item.cryptoListingCurrencyCode', app.polyglot.t('metadataModelErrors.provideCoinType'));
      }

      if (item) {
        if (typeof item.price !== 'undefined') {
          addError('item.price', 'The price should not be set on cryptocurrency ' +
            'listings.');
        }

        if (typeof item.condition !== 'undefined') {
          addError('item.condition', 'The condition should not be set on cryptocurrency ' +
            'listings.');
        }

        if (typeof item.quantity !== 'undefined') {
          addError('item.quantity', 'The quantity should not be set on cryptocurrency ' +
            'listings.');
        }

        if (
          item.cryptoListingPriceModifier === '' ||
          item.cryptoListingPriceModifier === undefined ||
          item.cryptoListingPriceModifier === null
        ) {
          addError('item.cryptoListingPriceModifier', app.polyglot.t('listingModelErrors.providePriceModifier'));
        } else if (typeof item.cryptoListingPriceModifier !== 'number') {
          addError('item.cryptoListingPriceModifier', app.polyglot.t('listingModelErrors.numericPriceModifier'));
        } else if (
          item.cryptoListingPriceModifier < this.max.minPriceModifier ||
          item.cryptoListingPriceModifier > this.max.maxPriceModifier
        ) {
          addError('item.cryptoListingPriceModifier', app.polyglot.t('listingModelErrors.priceModifierRange', {
            min: this.max.minPriceModifier,
            max: this.max.maxPriceModifier,
          }));
        }

        let coinDiv;
        try {
            coinDiv = getCoinDivisibility(item.cryptoListingCurrencyCode);
        } catch (e) {
          coinDiv = defaultCryptoCoinDivisibility;
        }
        this.validateCurrencyAmount(
          {
            amount: item.cryptoQuantity,
            currency: {
              code: () => item.cryptoListingCurrencyCode,
              divisibility: () => coinDiv,
            },
          },
          addError,
          'item.cryptoQuantity',
          {
            validationOptions: {
              rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
            },
          }
        );
      }
    } else {
      if (item && typeof item.cryptoQuantity !== 'undefined') {
        addError('item.cryptoQuantity', 'The cryptoQuantity should only be set on cryptocurrency ' +
          'listings.');
      }

      if (item && typeof item.cryptoListingCurrencyCode !== 'undefined') {
        addError('item.cryptoListingCurrencyCode', 'The cryptoListingCurrencyCode should only be set on cryptocurrency ' +
          'listings.');
      }

      this.validateCurrencyAmount(
        {
          amount: item.price,
          currency: curDefCurrency,
        },
        addError,
        'price'
      );

      (attrs.shippingOptions || []).forEach(shipOpt => {
        (shipOpt.services || []).forEach(service => {
          this.validateCurrencyAmount(
            {
              amount: service.price,
              currency: curDefCurrency,
            },
            addError,
            `shippingOptions[${shipOpt.cid}].services[${service.cid}].price`,
            {
              validationOptions: {
                rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
              },
            }
          );

          this.validateCurrencyAmount(
            {
              amount: service.additionalItemPrice,
              currency: curDefCurrency,
            },
            addError,
            `shippingOptions[${shipOpt.cid}].services[${service.cid}].additionalItemPrice`,
            {
              validationOptions: {
                rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
              },
            }
          );
        });
      });

      (item.skus || []).forEach(sku => {
        this.validateCurrencyAmount(
          {
            amount: sku.surcharge,
            currency: curDefCurrency,
          },
          addError,
          `item.skus[${sku.cid}].surcharge`,
          {
            validationOptions: {
              rangeType: CUR_VAL_RANGE_TYPES.GREATER_THAN_OR_EQUAL_ZERO,
            },
          }
        );
      });
    }

    if (attrs.coupons.length) {
      const coupons = attrs.coupons;

      if (coupons.length > this.max.couponCount) {
        addError('coupons', app.polyglot.t('listingModelErrors.tooManyCoupons',
          { maxCouponCount: this.max.couponCount }));
      }

      coupons.forEach(coupon => {
        const priceDiscount = coupon.priceDiscount;
        const itemPrice = item.price;

        this.validateCurrencyAmount(
          {
            amount: priceDiscount,
            currency: curDefCurrency,
          },
          addError,
          `coupons[${coupon.cid}].priceDiscount`,
          {
            translations: {
              required: false,
            },
          }
        );

        if (
          priceDiscount &&
          priceDiscount.isNaN &&
          !priceDiscount.isNaN() &&
          itemPrice &&
          itemPrice.isNaN &&
          !itemPrice.isNaN()
        ) {
          if (priceDiscount.gte(itemPrice)) {
            addError(`coupons[${coupon.cid}].priceDiscount`,
              app.polyglot.t('listingModelErrors.couponsPriceTooLarge'));
          }
        }
      });
    }

    errObj = this.mergeInNestedErrors(errObj);

    if (contractType === 'CRYPTOCURRENCY') {
      // Remove the validation of certain fields that should not be set for
      // cryptocurrency listings.
      Object
        .keys(errObj)
        .forEach(errKey => {
          if (errKey.startsWith('metadata.pricingCurrency')) {
            delete errObj[errKey];
          }
        });

      delete errObj['item.price'];
      delete errObj['item.condition'];
      delete errObj['item.quantity'];
      delete errObj['item.title'];
    } else {
      delete errObj['item.cryptoListingCurrencyCode'];
      delete errObj['item.cryptoListingPriceModifier'];
    }

    if (Object.keys(errObj).length) return errObj;

    return undefined;
  }

  fetch(options = {}) {
    if (
      options.cid !== undefined &&
      (
        typeof options.cid !== 'string' ||
        !options.cid
      )
    ) {
      throw new Error('If providing the options.cid, it must be a ' +
        'non-empty string.');
    }

    return super.fetch(options);
  }

  sync(method, model, options) {
    let returnSync = 'will-set-later';

    if (method === 'read') {
      if (!this.guid) {
        throw new Error('In order to fetch a listing, a guid must be set on the model instance.');
      }

      const slug = this.get('slug');

      if (!slug) {
        throw new Error('In order to fetch a listing, a slug must be set as a model attribute.');
      }

      options.url = options.url ||
        (
          typeof options.cid === 'string' && options.cid ?
            this.getIpfsUrl(options.cid) :
            this.getIpnsUrl(slug)
        );
    } else {
      if (method !== 'delete') {
        // it's a create or update

        options.url = options.url || app.getServerUrl('ob/listing');
        options.attrs = options.attrs || this.toJSON();

        let coinDiv;

        if (options.attrs.metadata.contractType !== 'CRYPTOCURRENCY') {
          // Don't send over crypto currency specific fields if it's not a
          // crypto listing.
          delete options.attrs.item.cryptoListingCurrencyCode;
          delete options.attrs.item.cryptoListingPriceModifier;

          coinDiv = options.attrs.metadata.pricingCurrency.divisibility;

          options.attrs.shippingOptions.forEach(shipOpt => {
            shipOpt.services.forEach(service => {
              service.price = decimalToInteger(
                service.price,
                coinDiv
              );
              service.additionalItemPrice =
                decimalToInteger(
                  service.additionalItemPrice,
                  coinDiv
                );
            });
          });

          options.attrs.coupons.forEach(coupon => {
            if (coupon.priceDiscount) {
              coupon.priceDiscount =
                decimalToInteger(coupon.priceDiscount, coinDiv);
            }
          });

          options.attrs.item.skus.forEach(sku => {
            sku.surcharge = decimalToInteger(sku.surcharge, coinDiv);
          });
        } else {
          // Don't send over the price on crypto listings.
          delete options.attrs.item.price;
          delete options.attrs.metadata.pricingCurrency;
          delete options.attrs.item.options;

          // Update the crypto title based on the accepted currency and
          // coin type.
          const coinType = options.attrs.item.cryptoListingCurrencyCode;
          let fromCur = options.attrs.metadata.acceptedCurrencies &&
            options.attrs.metadata.acceptedCurrencies[0];
          if (fromCur) {
            const curObj = getCryptoCurrencyByCode(fromCur);
            // if it's a recognized currency, ensure the mainnet code is used
            fromCur = curObj ? curObj.code : fromCur;
          } else {
            fromCur = 'UNKNOWN';
          }
          options.attrs.item.title = `${fromCur}-${coinType}`;
        }

        // If providing a quanitity, productID or infiniteInventory bool on the
        // Item and not providing any SKUs, then we'll send them in as a "dummy" SKU
        // (as the server expects).
        if (!options.attrs.item.skus.length) {
          const dummySku = {};

          if (options.attrs.metadata.contractType === 'CRYPTOCURRENCY') {
            dummySku.quantity = decimalToInteger(
              options.attrs.item.cryptoQuantity,
              options.attrs.metadata.coinDivisibility
            );

            delete options.attrs.item.cryptoQuantity;
          } else if (options.attrs.item.infiniteInventory) {
            dummySku.quantity = '-1';
          } else if (options.attrs.item.quantity instanceof bigNumber) {
            dummySku.quantity = options.attrs.item.quantity;
          }

          if (
            options.attrs.metadata.contractType !== 'CRYPTOCURRENCY' &&
            typeof options.attrs.item.productID === 'string' &&
            options.attrs.item.productID.length
          ) {
            dummySku.productID = options.attrs.item.productID;
          }

          if (Object.keys(dummySku).length) {
            options.attrs.item.skus = [dummySku];
          }

          delete options.attrs.item.infiniteInventory;
        }

        delete options.attrs.item.productID;
        delete options.attrs.item.quantity;

        // Our Sku has an infinteInventory boolean attribute, but the server
        // is expecting a quantity negative quantity in that case.
        options.attrs.item.skus.forEach(sku => {
          if (sku.infiniteInventory) {
            sku.quantity = bigNumber('-1');
          }

          delete sku.infiniteInventory;
        });

        // remove the hash
        delete options.attrs.cid;

        // If all countries are individually provided as shipping regions, we'll send
        // 'ALL' to the server.
        options.attrs.shippingOptions.forEach(shipOpt => {
          if (_.isEqual(Object.keys(getIndexedCountries()), shipOpt.regions)) {
            shipOpt.regions = ['ALL'];
          }
        });

        if (app.serverConfig.testnet) {
          options.attrs.metadata.escrowTimeoutHours =
            options.attrs.metadata.escrowTimeoutHours === undefined ?
              1 : options.attrs.metadata.escrowTimeoutHours;
        }
      } else {
        options.url = options.url ||
          app.getServerUrl(`ob/listing/${this.get('slug')}`);
      }
    }

    returnSync = super.sync(method, model, options);

    const eventOpts = {
      xhr: returnSync,
      url: options.url,
    };

    if (method === 'create' || method === 'update') {
      const attrsBeforeSync = this.lastSyncedAttrs;

      returnSync.done(data => {
        const hasChanged = () => (!_.isEqual(attrsBeforeSync, this.toJSON()));

        if (data.slug) {
          this.set('slug', data.slug);
        }

        listingEvents.trigger('saved', this, {
          ...eventOpts,
          created: method === 'create',
          slug: this.get('slug'),
          prev: attrsBeforeSync,
          hasChanged,
        });
      });
    } else if (method === 'delete') {
      listingEvents.trigger('destroying', this, {
        ...eventOpts,
        slug: this.get('slug'),
      });

      returnSync.done(() => {
        listingEvents.trigger('destroy', this, {
          ...eventOpts,
          slug: this.get('slug'),
        });
      });
    }

    return returnSync;
  }

  parse(response) {
    this.unparsedResponse = JSON.parse(JSON.stringify(response)); // deep clone
    const parsedResponse = response.listing;

    if (parsedResponse) {
      const isCrypto = parsedResponse.metadata &&
        parsedResponse.metadata.contractType === 'CRYPTOCURRENCY';

      // set the cid
      parsedResponse.cid = response.cid;

      let currencyCode;
      try {
        currencyCode = isCrypto ? parsedResponse.item.cryptoListingCurrencyCode : parsedResponse.metadata.pricingCurrency.code;
      } catch (e) {
        // pass
      }

      let coinDiv;
      try {
        coinDiv = getCoinDivisibility(currencyCode);
      } catch (e) {
        // pass
      }

      const [isValidCoinDiv] = isValidCoinDivisibility(coinDiv);

      if (!isValidCoinDiv) {
        console.error('Unable to convert price fields. The coin divisibility is not valid.');
      }

      if (!isCrypto) {
        if (parsedResponse.item) {
          delete parsedResponse.item.cryptoListingCurrencyCode;
          delete parsedResponse.item.cryptoListingPriceModifier;
        }

        if (parsedResponse.item) {
          parsedResponse.item.price =
            integerToDecimal(
              parsedResponse.item.price,
              coinDiv,
              { fieldName: 'item.price' }
            );
        }

        if (parsedResponse.shippingOptions && parsedResponse.shippingOptions.length) {
          parsedResponse.shippingOptions.forEach((shipOpt, shipOptIndex) => {
            if (shipOpt.services && shipOpt.services.length) {
              shipOpt.services.forEach(service => {
                service.price = integerToDecimal(
                  service.price,
                  coinDiv,
                  { fieldName: 'service.price' }
                );
                service.additionalItemPrice =
                  integerToDecimal(
                    service.additionalItemPrice,
                    coinDiv,
                    { fieldName: 'service.additionalItemPrice' }
                  );
              });
            }

            // If the shipping regions are set to 'ALL', we'll replace with a list of individual
            // countries, which is what our UI is designed to work with.
            if (shipOpt.regions && shipOpt.regions.length && shipOpt.regions[0] === 'ALL') {
              parsedResponse.shippingOptions[shipOptIndex].regions =
                Object.keys(getIndexedCountries());
            }
          });
        }

        if (parsedResponse.coupons) {
          parsedResponse.coupons.forEach(coupon => {
            if (coupon.priceDiscount) {
              coupon.priceDiscount =
                integerToDecimal(
                  coupon.priceDiscount,
                  coinDiv,
                  { fieldName: 'coupon.priceDiscount' }
                );
            }
          });
        }
      }

      // Re-organize variant structure so a "dummy" SKU (if present) has its quanitity
      // and productID moved to be attributes of the Item model
      if (
        parsedResponse.item && parsedResponse.item.skus &&
        parsedResponse.item.skus.length === 1 &&
        typeof parsedResponse.item.skus[0].selections === 'undefined'
      ) {
        const dummySku = parsedResponse.item.skus[0];

        if (isCrypto) {
          parsedResponse.item.cryptoQuantity = integerToDecimal(
            dummySku.quantity,
            coinDiv,
            { fieldName: 'sku.quantity' }
          );
        } else {
          parsedResponse.item.quantity = dummySku.quantity;
        }

        parsedResponse.item.productID = dummySku.productID;
        delete parsedResponse.item.skus;
      } else if (parsedResponse.item && parsedResponse.item.skus) {
        parsedResponse.item.skus.forEach(sku => {
          // If a sku quantity is set to less than 0, we'll set the
          // infinite inventory flag.
          if (bigNumber(sku.quantity).lt(0)) {
            sku.infiniteInventory = true;
          } else {
            sku.infiniteInventory = false;
          }

          // convert the surcharge
          const surcharge = sku.surcharge;

          if (surcharge) {
            sku.surcharge = integerToDecimal(
              surcharge,
              coinDiv,
              { fieldName: 'sku.surcharge' }
            );
          }
        });
      }

      if (parsedResponse.metadata) {
        parsedResponse.metadata.acceptedCurrencies =
          parsedResponse.metadata.acceptedCurrencies || [];
      }
    }

    return parsedResponse;
  }
}
