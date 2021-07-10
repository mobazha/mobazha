import { Platform } from 'react-native';

import { searchAPI } from './const';
import { filteroutCryptoFromSearch } from '../utils/listings';

export function getRandomSearch(query, queryString, page = 0, numPerPage = 40, categories) {
  const qVal = query === '' ? '*' : query;
  let apiUrl = `${searchAPI}/listings?ps=${numPerPage}&q=${qVal}&${queryString}&page=${page}`;
  if (Platform.OS === 'ios') {
    apiUrl = `${apiUrl}&mobile&type=physical_good&type=service`;
  }
  if (categories) {
    apiUrl = `${apiUrl}&categories=${encodeURI(categories)}`;
  }
  return fetch(apiUrl)
    .then(response => (response.json()))
    .then(filteroutCryptoFromSearch);
}

// Fetch search results from a query
export function getSearchResult(query, queryString, page = 0, numPerPage = 40, categories) {
  const qVal = query === '' ? '*' : query;
  let apiUrl = `${searchAPI}/listings?ps=${numPerPage}&q=${encodeURI(qVal)}&${queryString}&p=${page}`;
  if (Platform.OS === 'ios') {
    apiUrl = `${apiUrl}&mobile&type=physical_good&type=service`;
  }
  if (categories) {
    apiUrl = `${apiUrl}&categories=${encodeURI(categories)}`;
  }
  return fetch(apiUrl)
    .then(response => (response.json()))
    .then(filteroutCryptoFromSearch);
}

// Fetch search results from a listing
export function getListingResult(queryString, page = 0, numPerPage = 40) {
  let apiUrl = `${searchAPI}/listings?ps=${numPerPage}&${queryString}&p=${page}`;
  if (Platform.OS === 'ios') {
    apiUrl = `${apiUrl}&mobile&type=physical_good&type=service`;
  }
  return fetch(apiUrl)
    .then(response => (response.json()))
    .then(filteroutCryptoFromSearch)
    .catch((err) => {
      console.log(err);
      return { results: [] };
    });
}

// Fetch search results for profile
export function searchProfile(keyword, page = 0, numPerPage = 40) {
  const qVal = keyword === '' ? '*' : keyword;
  let apiUrl = `${searchAPI}/vendors?ps=${numPerPage}&page=${page}&q=${qVal}`;
  if (Platform.OS === 'ios') {
    apiUrl = `${apiUrl}&mobile`;
  }
  return fetch(apiUrl)
    .then(response => (response.json()))
    .catch((err) => {
      console.log(err);
      return { results: [] };
    });
}
