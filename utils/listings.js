import { Platform } from 'react-native';

import {I18n} from '../langs/I18n';

export const PREVIEWING_CATEGORIES = Platform.select({
  ios: [
    { title: I18n.t('utils.listings.Electronics'), shortName: 'electronics', categoryName: 'Consumer Electronics' },
    { title: I18n.t('utils.listings.Women_Clothing'), shortName: 'wclothing', categoryName: "Women's Clothing" },
    { title: I18n.t('utils.listings.Art'), shortName: 'art', categoryName: "Art" },
    // { title: I18n.t('utils.listings.Men_Clothing'), shortName: 'mclothing', categoryName: "Men's Clothing" },
    // { title: I18n.t('utils.listings.Toys_Games'), shortName: 'toy', categoryName: 'Toys & Hobbies' },
    // { title: I18n.t('utils.listings.Jewelry'), shortName: 'jewelry', categoryName: 'Jewelry & Accessories' },
    { title: I18n.t('utils.listings.Tools'), shortName: 'tools', categoryName: 'Tools' },
  ],
  android: [
    { title: I18n.t('utils.listings.Electronics'), shortName: 'electronics', categoryName: 'Consumer Electronics' },
    // { title: I18n.t('utils.listings.Gift_Cards'), shortName: 'giftcards', categoryName: 'Gift Cards' },
    { title: I18n.t('utils.listings.Women_Clothing'), shortName: 'wclothing', categoryName: "Women's Clothing" },
    { title: I18n.t('utils.listings.Art'), shortName: 'art', categoryName: "Art" },
    // { title: I18n.t('utils.listings.Men_Clothing'), shortName: 'mclothing', categoryName: "Men's Clothing" },
    // { title: I18n.t('utils.listings.Toys_Games'), shortName: 'toy', categoryName: 'Toys & Hobbies' },
    // { title: I18n.t('utils.listings.Jewelry'), shortName: 'jewelry', categoryName: 'Jewelry & Accessories' },
    { title: I18n.t('utils.listings.Tools'), shortName: 'tools', categoryName: 'Tools' },
  ],
});

export const filteroutCryptoFromListings = (json) => {
  const items = json;
  return items.filter(item => item.contractType !== 'CRYPTOCURRENCY');
};

export const filteroutCryptoFromSearch = (json) => {
  const { results, ...others } = json;
  const { results: items, ...restOfResults } = results;
  return {
    ...others,
    results: {
      ...restOfResults,
      results: items.filter(item => item.data.contractType !== 'CRYPTOCURRENCY'),
    },
  };
};

export const shuffle = (array) => {
  const cloned = [...array];
  for (let i = cloned.length - 1; i > 0; i -= 1) {
    const j = Math.floor(Math.random() * (i + 1));
    [cloned[i], cloned[j]] = [cloned[j], cloned[i]];
  }
  return cloned;
};
