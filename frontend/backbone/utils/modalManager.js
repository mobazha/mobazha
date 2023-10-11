import _ from 'underscore';
import app from '../app';
import Listing from '../models/listing/Listing';
import About from '../views/modals/about/About';
import DebugLog from '../views/modals/DebugLog';
import ModeratorDetails from '../views/modals/moderatorDetails';
import Wallet from '../views/modals/wallet/Wallet';

let aboutModal;
let settingsModal;
let debugLogModal;
let moderatorDetailsModal;
let _wallet;

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
  if (moderatorDetailsModal) moderatorDetailsModal.remove();

  moderatorDetailsModal = new ModeratorDetails(modalOptions)
      .render()
      .open();

  return moderatorDetailsModal;
}

export function launchWallet(modalOptions = {}) {
  // if (_wallet) {
  //   _wallet.open();
  // } else {
  //   _wallet = new Wallet({
  //     removeOnRoute: false,
  //     ...modalOptions,
  //   })
  //     .render()
  //     .open();

  //   app.router.on('will-route', () => _wallet.close());
  // }
  // return _wallet;

  _wallet = app.router.loadVueModal('Wallet');

  return _wallet;
}

export function getWallet() {
  return _wallet;
}
