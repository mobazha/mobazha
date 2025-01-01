/* eslint-disable class-methods-use-this */
import $ from 'jquery';
import { Router } from 'backbone';
import * as isIPFS from 'is-ipfs';
import app from './app';
import { isPromise } from './utils/object';
import './lib/whenAll.jquery';
import { getOpenModals } from './views/modals/BaseModal';

// 路由工具类
class RouterUtils {
  static standardizeRoute(route) {
    let standardized = route;
    if (standardized.startsWith('#')) standardized = standardized.slice(1);
    if (standardized.startsWith('/')) standardized = standardized.slice(1); 
    if (standardized.startsWith('ob://')) standardized = standardized.slice(5);
    if (standardized.endsWith('/')) standardized = standardized.slice(0, -1);
    return standardized;
  }

  static isValidUserRoute(guid, state, deepRouteParts) {
    const validStates = ['home', 'store', 'following', 'followers', 'reputation'];
    if (!guid || !validStates.includes(state)) return false;
    
    if (state === 'store') {
      return deepRouteParts.length <= 1;
    }
    return deepRouteParts.length === 0;
  }
}

export default class ObRouter extends Router {
  constructor(options = {}) {
    super(options);
    this.options = options;

    // This is a mapping of guids to handles. It is currently updated any time
    // a profile is fetched via this.user() and anytime a user route is navigated to
    // via this.navigateUser(). The main purpose of this cache is to avoid the flicker
    // in the address bar that would be present due to the fact that we are storing user
    // routes with guids in the history, but diplaying a version with the handle in the
    // address bar.
    this.guidHandleMap = new Map();
  }

  get maxCachedHandles() {
    return 1000;
  }

  // FYI - There is a scenario where the prevHash will be inaccurate. More details in
  // the confirmPromises when() fail handler in execute().
  setPrevHash(prevHash = this._curHash) {
    this._prevHash = prevHash;
    this._curHash = location.hash;
  }

  get prevHash() {
    return this._prevHash;
  }

  /**
   * Updates our this.guidHandleMap which is an in-memory mapping of a guid to handle.
   */
  cacheGuidHandle(guid, handle) {
    if (typeof guid !== 'string') {
      throw new Error('Please provide a guid as a string.');
    }

    if (typeof handle !== 'string') {
      throw new Error('Please provide a handle as a string.');
    }

    if (!handle) {
      this.guidHandleMap.delete(guid);
      return;
    }

    const keys = Array.from(this.guidHandleMap.keys());
    if (!this.guidHandleMap.get(guid) && keys.length >= this.maxCachedHandles) {
      // We're already at or over the limit, so we need to remove one from the cache to
      // make room for the new one.
      this.guidHandleMap.delete(keys[0]);
    }

    this.guidHandleMap.set(guid, handle);
  }

  standardizedRoute(route = location.hash) {
    return RouterUtils.standardizeRoute(route);
  }

  setAddressBarText(route = location.hash) {
    let displayRoute = this.standardizedRoute(route);

    if (!route) {
      displayRoute = '';
    } else {
      const split = route.split('/');

      // If the route starts with a guid and we have a cached handle
      // for that guid, we'll put the handle in.
      if (isIPFS.multihash(split[0])) {
        const handle = this.guidHandleMap.get(split[0]);

        if (handle) {
          displayRoute = `@${handle}${split.length > 1 ? `/${split.slice(1).join('/')}` : ''}`;
        }
      }

      displayRoute = `ob://${displayRoute}`;
    }

    app.pageNav.setAddressBar(displayRoute);
  }

  execute(callback, args, name, options = {}) {
    if (this.closeUnconfirmedRollBack) {
      this.closeUnconfirmedRollBack = false;
      return false;
    }

    this.navigate(this.standardizedRoute(), { replace: true });

    // We'll iterate through any open modal which have a confirmClose method
    // implemented. We'll call the method and only proceed with the route
    // if every method confirms that the close is ok. If not, we'll cancel
    // the route and roll back the hash.
    if (!options.confirmedClose) {
      const confirmPromises = [];
      getOpenModals().forEach((modal) => {
        if (typeof modal.confirmClose !== 'function') return;
        const closeConfirmed = modal.confirmClose.call(modal);

        if (isPromise(closeConfirmed)) {
          confirmPromises.push(closeConfirmed);
        } else if (closeConfirmed) {
          confirmPromises.push($.Deferred().resolve().promise());
        } else {
          confirmPromises.push($.Deferred().reject().promise());
        }
      });

      if (confirmPromises.length) {
        // Routing to a new page while the confirm close process is active could produce
        // weird things, so we'll block page navigation.
        app.pageNav.navigable = false;
        $.when(...confirmPromises)
          .done(() => {
            this.execute(callback, args, name, { confirmedClose: true });
          })
          .fail(() => {
            // If any of the closeConfirm promises are rejected, it indicates that
            // the close of at least one modal was not confirmed and we won't proceed
            // with the new route. We need to rollback the location hash.

            if (location.hash !== this._prevHash) {
              // When we roll back, it will trigger a new route. We want that route to be
              // ignored and not reload a new page since we never unloaded the page. It's not
              // pretty, but the following flag will be used for execute() to opt-out of reloading
              // the page.
              this.closeUnconfirmedRollBack = true;
              location.hash = this._prevHash;

              // FYI - at this point, since we've rolled back one level but never rolled back
              // _prevHash one level, _prevHash is not accurate. To do that, we would need to track
              // more than the previous hash, but also track all previous hashes. It's beyond the
              // scope of what is necessary here. As long as prev hash is used only when the
              // location hash changes to a new one and you want to cancel that route, we're good.
            }
          })
          .always(() => {
            app.pageNav.navigable = true;
          });

        return false;
      }
    }

    app.loadingModal.open();

    // This block is intentionally duplicated here and in loadPage. It's
    // here because we want to remove any current views (and have them
    // do their remove cleanup) as soon as we know we're matching a new
    // route. Based on some subsequent async fecthes, it may be a little
    // bit of time before loadPage is called.
    if (this.currentPage) {
      this.currentPage.remove();
      this.currentPage = null;
    }

    if (callback) {
      this.trigger('will-route');
      callback.apply(this, args);
    }

    return undefined;
  }

  /**
   * If you need to navigate to a user page via a handle and you have the user's guid, use
   * this method which is mostly a wrapper around the standard Router.navigate. The addition
   * is that this will make sure to store a version of the given fragment in history with the
   * guid in place of the handle. It will make sure that the given version (with handle)
   * will be shown in the address bar. It will also update the guidHandleMap caching.
   *
   * It's essentially a way to ensure that behind the scenes navigation is being done via
   * guids (not dependant on the 3rd party resolver), but visually in the address bar, the
   * user is seeing the handle (when available).
   *
   * @param {string} fragment - The user route you want stored. If the handle is available,
   *   provide it in this parameter (e.g. '@themes/store')
   * @param {guid} string - The guid of the user corresponding to the given fragment.
   * @param {object} [options={}] - Options that will be passed to Router.navigate.
   */
  navigateUser(fragment, guid, options = {}) {
    if (typeof fragment !== 'string') {
      throw new Error('Please provide a fragment as a string.');
    }

    if (!guid) {
      throw new Error('Please provide a guid.');
    }

    let guidRoute = fragment;
    const split = fragment.split('/');

    if (split[0].startsWith('@')) {
      this.cacheGuidHandle(guid, split[0].slice(1));
      guidRoute = [guid].concat(split.slice(1)).join('/');
    }

    return this.navigate(guidRoute, options);
  }

  navigate(fragment, options = {}) {
    // Navigate is often times called in quick succession with url rewrites, so to
    // properly capture the previous hash we'll just base it off the final call when
    // they're called in such a burst fashion.
    if (typeof fragment === 'string') {
      clearTimeout(this.navigateSetPrevHash);
      this.navigateSetPrevHash = setTimeout(() => {
        this.setPrevHash();
      });
    }

    console.log('fragment: ', fragment)
    return super.navigate(fragment.replace(/^(ob:\/\/)/, ''), options);
  }

  userViaHandle(handle, ...args) {
    // getGuid(handle).done((guid) => {
    //   // hack to pass in the handle to this.user - forgive me code gods
    //   this.user(guid, ...[args[0], { handle }, ...args.slice(1)]);
    // }).fail(() => {
    //   this.userNotFound(handle);
    // });
  }

}
