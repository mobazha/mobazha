import _ from 'underscore';
import app from '../app';
import Profile from '../models/profile/Profile';
import Listing from '../models/listing/Listing';
import About from '../views/modals/about/About';
import DebugLog from '../views/modals/DebugLog';

let aboutModal;
let debugLogModal;

export function launchEditListingModal(modalOptions = {}) {
  const model = modalOptions.model;
  if (!(model instanceof Listing)) {
    throw new Error('In the modalOptions, please provide an instance of ' +
      'a Listing model.');
  }

  return window.vueApp.launchModal('EditListing', _.omit(modalOptions, 'model'), function() {
      return {
        model,
      };
    });
}

export function launchAboutModal(modalOptions = {}) {
  if (aboutModal) {
    aboutModal.bringToTop();
    if (modalOptions.initialTab) aboutModal.selectTab(modalOptions.initialTab);
  } else {
    aboutModal = new About({
      removeOnClose: true,
      ...modalOptions,
    })
      .render()
      .open();

    aboutModal.on('modal-will-remove', () => (aboutModal = null));
  }

  return aboutModal;
}

export function launchSettingsModal(modalOptions = {}) {
  return window.vueApp.launchModal('Settings', modalOptions);
}

export function launchDebugLogModal(modalOptions = {}) {
  if (debugLogModal) debugLogModal.remove();

  debugLogModal = new DebugLog(modalOptions)
    .render()
    .open();

  return debugLogModal;
}

export function launchModeratorDetailsModal(modalOptions = {}) {
  const model = modalOptions.model;
  if (!(model instanceof Profile)) {
    throw new Error('In the modalOptions, please provide an instance of ' +
      'a Profile model.');
  }

  return window.vueApp.launchModal('ModeratorDetails', _.omit(modalOptions, 'model'), function() {
      return {
        model,
      };
    });
}

export function launchWallet(modalOptions = {}) {
  return window.vueApp.launchModal('Wallet', modalOptions, function() {
    return {
      walletBalances: app.walletBalances,
    };
  });
}
