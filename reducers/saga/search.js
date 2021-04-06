import { call, put, takeEvery, takeLatest, select } from 'redux-saga/effects';
import { stringify } from 'query-string';
import { get, endsWith } from 'lodash';

import { actions } from '../search';
import { actions as profilesActions } from '../profiles';
import { getSearchResult, searchProfile } from '../../api/search';
import { getPageNum, hasMore, getSearchFilter, getKeyword, getUserPageNum, hasMoreUser, getBlockedNodes, getCategory } from './sagaSelector';

import { parseSearchFilter } from '../../utils/dataMapping';

function* fetchSearchResult() {
  try {
    const morePages = yield select(hasMore);
    const searchFilter = yield select(getSearchFilter);
    const keyword = yield select(getKeyword);
    const category = yield select(getCategory);
    const blockedNodes = yield select(getBlockedNodes);

    const targetFilter = parseSearchFilter(searchFilter);
    const queryString = stringify(targetFilter);
    if (morePages) {
      const curPage = yield select(getPageNum);
      const result = yield call(getSearchResult, keyword, queryString, curPage, 40, category);
      yield put({
        type: actions.setSearchResult,
        payload: {
          ...result.results,
          results: result.results.results.filter(
            listingInfo => !blockedNodes.includes(get(listingInfo, 'relationships.vendor.data.peerID')),
          ),
        },
      });
    }
  } catch (err) {
    console.log(err);
    yield put({
      type: actions.setSearchResult,
      payload: { total: null, morePages: false, results: [] },
    });
  }
}

function* fetchUserSearchResult() {
  const morePages = yield select(hasMoreUser);
  const keyword = yield select(getKeyword);
  const blockedNodes = yield select(getBlockedNodes);
  if (morePages) {
    const curPage = yield select(getUserPageNum);
    const result = yield call(searchProfile, keyword, curPage);
    const searchResults = result.results.results? result.results.results.filter(
      userInfo => !endsWith(get(userInfo, 'data.userAgent'), 'could not resolve name\n'),
    ).filter(
      userInfo => !blockedNodes.includes(get(userInfo, 'data.peerID')),
    ):[];
    yield put({
      type: actions.setUserSearchResult,
      payload: { ...result.results, results: searchResults },
    });
    yield put({
      type: profilesActions.setProfiles,
      payload: result.results.results? result.results.results.map(result => get(result, 'data')):[],
    });
  }
}

function* updateFilter() {
  yield put({ type: actions.doSearch });
}

const SearchSaga = function* Search() {
  yield takeLatest(actions.doSearch, fetchSearchResult);
  yield takeLatest(actions.doSearchUser, fetchUserSearchResult);
  yield takeLatest(actions.appendSearch, fetchSearchResult);
  yield takeLatest(actions.appendUserSearch, fetchUserSearchResult);
  yield takeEvery(actions.updateFilter, updateFilter);
};

export default SearchSaga;
