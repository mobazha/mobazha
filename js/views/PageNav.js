import { remote } from 'electron';
import { isMultihash } from '../utils';
import { events as serverConnectEvents, getCurrentConnection } from '../utils/serverConnect';
import { setUnreadNotifCount, launchNativeNotification } from '../utils/notification';
import { recordEvent } from '../utils/metrics';
import Backbone from 'backbone';
import BaseVw from './baseVw';
import loadTemplate from '../utils/loadTemplate';
import app from '../app';
import $ from 'jquery';
import {
  launchEditListingModal, launchAboutModal,
  launchWallet, launchSettingsModal,
} from '../utils/modalManager';
import Listing from '../models/listing/Listing';
import { getAvatarBgImage } from '../utils/responsive';
import PageNavServersMenu from './PageNavServersMenu';
import AddressBarIndicators from './AddressBarIndicators';
import { getNotifDisplayData } from '../collections/Notifications';
import Notifications from './notifications/Notificiations';

export default class extends BaseVw {
  constructor(options) {
    const opts = {
      events: {
        'click .js-navBack': 'navBackClick',
        'click .js-navFwd': 'navFwdClick',
        'click .js-navReload': 'navReload',
        'click .js-navClose': 'navCloseClick',
        'click .js-navMin': 'navMinClick',
        'click .js-navMax': 'navMaxClick',
        'keyup .js-addressBar': 'onKeyupAddressBar',
        'focusin .js-addressBar': 'onFocusInAddressBar',
        'click .js-navListBtn': 'navListBtnClick',
        'click .js-navSettings': 'navSettingsClick',
        'click .js-navHelp': 'navHelpClick',
        'click .js-navAboutModal': 'navAboutClick',
        'click .js-navWalletBtn': 'navWalletClick',
        'click .js-navCreateListing': 'navCreateListingClick',
        'click .js-navListItem': 'onNavListItemClick',
        'click .js-navList': 'onNavListClick',
        'mouseenter .js-connectedServerListItem': 'onMouseEnterConnectedServerListItem',
        'mouseleave .js-connectedServerListItem': 'onMouseLeaveConnectedServerListItem',
        'mouseenter .js-connManagementContainer': 'onMouseEnterConnManagementContainer',
        'mouseleave .js-connManagementContainer': 'onMouseLeaveConnManagementContainer',
        'click .js-navNotifBtn': 'onClickNavNotifBtn',
        'click .js-notifContainer': 'onClickNotifContainer',
        'click .js-notificationListItem a[href]': 'onClickNotificationLink',
      },
      navigable: false,
      ...options,
    };

    if (!opts.serverConfigs) {
      throw new Error('Please provide a Server Configs collection');
    }

    opts.className = 'pageNav';
    if (!opts.navigable) opts.className += ' notNavigable';
    if (opts.torIndicatorOn) opts.className += ' torIndicatorOn';
    super(opts);
    this.options = opts;
    this.addressBarText = '';

    this.boundOnDocClick = this.onDocClick.bind(this);
    $(document).on('click', this.boundOnDocClick);

    this.listenTo(app.localSettings, 'change:windowControlStyle',
      (_, style) => this.setWinControlsStyle(style));
    this.setWinControlsStyle(app.localSettings.get('windowControlStyle'));

    this.listenTo(serverConnectEvents, 'connected', e => {
      this.$connectedServerName.text(e.server.get('name'))
        .addClass('txB');
      this.listenTo(app.router, 'route:search', this.onRouteSearch);
      this.fetchUnreadNotifCount().done(data => {
        this.unreadNotifCount = (this.unreadNotifCount || 0) + data.unread;
      });
      this.listenTo(e.socket, 'message', this.onSocketMessage);
    });

    this.listenTo(serverConnectEvents, 'disconnected', e => {
      this.$connectedServerName.text(app.polyglot.t('pageNav.notConnectedMenuItem'))
        .removeClass('txB');
      this.torIndicatorOn = false;
      this.stopListening(app.router, null, this.onRouteSearch);
      this.getCachedEl('.js-notifUnreadBadge').addClass('hide');
      this.stopListening(e.socket, 'message', this.onSocketMessage);
    });
  }

  onSocketMessage(e) {
    const notif = e.jsonData.notification;

    if (notif) {
      if (notif.type === 'unfollow') return;
      this.unreadNotifCount = (this.unreadNotifCount || 0) + 1;
      setUnreadNotifCount(this.unreadNotifCount);

      const notifDisplayData = getNotifDisplayData(notif, { native: true });
      const nativeNotifData = {
        silent: true,
        onclick: () => {
          remote.getCurrentWindow().restore();

          if (notifDisplayData.route) {
            location.hash = notifDisplayData.route;
          }
        },
      };

      if (notif.thumbnail) {
        nativeNotifData.icon = app.getServerUrl(`ob/image/${notif.thumbnail.small}`);
      }

      launchNativeNotification(notifDisplayData.text, nativeNotifData);
    }
  }

  get navigable() {
    return this.options.navigable;
  }

  set navigable(navigable) {
    const prevNavigable = this.options.navigable;

    this.options.navigable = !!navigable;

    if (this.options.navigable !== prevNavigable) {
      if (this.options.navigable) {
        this.$el.removeClass('notNavigable');
      } else {
        this.$el.addClass('notNavigable');
      }
    }
  }

  get torIndicatorOn() {
    return this.options.torIndicatorOn;
  }

  set torIndicatorOn(bool) {
    if (this.options.torIndicatorOn !== bool) {
      this.options.torIndicatorOn = bool;
      this.$el.toggleClass('torIndicatorOn', bool);
    }
  }

  navBackClick() {
    recordEvent('NavClick', { target: 'back' });
    window.history.back();
  }

  navFwdClick() {
    recordEvent('NavClick', { target: 'forward' });
    window.history.forward();
  }

  navReload() {
    app.loadingModal.open();

    // Introducing some fake latency to ensure the loading modal has a chance
    // to appear. Otherwise, views that render quickly (e.g. have cached data)
    // load so fast it may look like pressing the refresh button did nothing.
    setTimeout(() => {
      Backbone.history.loadUrl();
    }, 200);
  }

  get unreadNotifCount() {
    return this._unreadNotifCount;
  }

  set unreadNotifCount(count) {
    if (typeof count !== 'number') {
      throw new Error('Please provide a count as a number.');
    }

    if (count === this._unreadNotifCount) return;
    this._unreadNotifCount = count;
    this.renderUnreadNotifCount();
    setUnreadNotifCount(this.unreadNotifCount);
  }

  fetchUnreadNotifCount() {
    if (this.unreadNotifCountFetch) this.unreadNotifCountFetch.abort();

    // We'll send a bogus filter because all we want is the count - we don't
    // want to weight the returned payload down with any notifications. Those
    // will be lazy loaded in when the notif menu is opened.
    return $.get(app.getServerUrl('ob/notifications?filter=blah-blah'));
  }

  renderUnreadNotifCount() {
    this.getCachedEl('.js-notifUnreadBadge')
      .toggleClass('hide', !this.unreadNotifCount)
      .toggleClass('ellipsisShown', this.unreadNotifCount > 99)
      .text(this.unreadNotifCount > 99 ? '…' : this.unreadNotifCount);
  }

  setWinControlsStyle(style) {
    if (style !== 'mac' && style !== 'win') {
      throw new Error('Style must be \'mac\' or \'win\'.');
    }

    this.$el.removeClass('winStyleWindowControls macStyleWindowControls');
    this.$el.addClass(style === 'mac' ? 'macStyleWindowControls' : 'winStyleWindowControls');
  }

  setAppProfile() {
    // when this view is created, the app.profile doesn't exist
    this.listenTo(app.profile.get('avatarHashes'), 'change', this.updateAvatar);
    this.render();
  }

  updateAvatar() {
    this.$('#AvatarBtn').attr('style', getAvatarBgImage(app.profile.get('avatarHashes').toJSON()));
  }

  navCloseClick() {
    recordEvent('NavClick', { target: 'close' });
    if (remote.process.platform !== 'darwin') {
      remote.getCurrentWindow().close();
    } else {
      remote.getCurrentWindow().hide();
    }
  }

  navMinClick() {
    recordEvent('NavClick', { target: 'minimize' });
    remote.getCurrentWindow().minimize();
  }

  navMaxClick() {
    recordEvent('NavClick', { target: 'maximize' });
    remote.getCurrentWindow().setFullScreen(!remote.getCurrentWindow().isFullScreen());
  }

  onRouteSearch() {
    const connectedServer = getCurrentConnection();

    if (connectedServer && connectedServer.server) {
      connectedServer.server.save({ dismissedDiscoverCallout: true });
    }

    this.getCachedEl('.js-discoverCallout').remove();
  }

  onMouseEnterConnectedServerListItem() {
    this.overConnectedServerListItem = true;
    this.$connManagementContainer.addClass('open');
  }

  onMouseLeaveConnectedServerListItem() {
    this.overConnectedServerListItem = false;

    setTimeout(() => {
      if (!this.overConnManagementContainer) {
        this.$connManagementContainer.removeClass('open');
      }
    }, 100);
  }

  onMouseEnterConnManagementContainer() {
    this.overConnManagementContainer = true;
  }

  onMouseLeaveConnManagementContainer() {
    this.overConnManagementContainer = false;

    setTimeout(() => {
      if (!this.overConnectedServerListItem) {
        this.$connManagementContainer.removeClass('open');
      }
    }, 100);
  }

  onNavListItemClick() {
    // Set timeout allows the new page to show before the overlay is removed. Otherwise,
    // there's a flicker frmo the old page to the new page.
    setTimeout(() => {
      this.closeNavMenu();
    });
  }

  navListBtnClick(e) {
    this.closeNotifications({
      closeOverlay: false,
      closeNavList: false,
    });
    this.toggleNavMenu();
    // do not bubble to onDocClick
    e.stopPropagation();
  }

  toggleNavMenu() {
    const isOpen = this.$navList.hasClass('open');
    this.$navList.toggleClass('open', !isOpen);
    this.$navOverlay.toggleClass('open', !isOpen);

    if (!isOpen) {
      this.$connManagementContainer.removeClass('open');
      recordEvent('NavClick', { target: 'navMenuOpen' });
    }
  }

  closeNavMenu() {
    this.$navList.removeClass('open');
    this.$navOverlay.removeClass('open');
    this.$connManagementContainer.removeClass('open');
  }

  onNavListClick(e) {
    // do not bubble to onDocClick
    e.stopPropagation();
  }

  onClickNavNotifBtn(e) {
    this.$navList.removeClass('open');
    this.$connManagementContainer.removeClass('open');
    this.toggleNotifications();
    // do not bubble to onDocClick
    e.stopPropagation();
  }

  isNotificationsOpen() {
    return this.getCachedEl('.js-notifContainer').hasClass('open');
  }

  toggleNotifications() {
    if (this.isNotificationsOpen()) {
      this.closeNotifications();
      this.$navOverlay.removeClass('open');
    } else {
      this.$navOverlay.addClass('open');
      recordEvent('NavClick', { target: 'notificationsOpen' });

      // open notifications menu
      if (!this.notifications) {
        this.notifications = new Notifications();
        this.getCachedEl('.js-notifContainer').html(this.notifications.render().el);
        this.listenTo(this.notifications, 'notifNavigate', () => this.closeNotifications());
      }

      this.getCachedEl('.js-notifContainer').addClass('open');
    }
  }

  onClickNotifContainer(e) {
    // do not bubble to onDocClick
    e.stopPropagation();
  }

  closeNotifications(options) {
    const opts = {
      closeOverlay: true,
      closeNavList: true,
      ...options,
    };

    if (!this.isNotificationsOpen()) return;
    if (opts.closeNavList) this.$navList.removeClass('open');
    this.getCachedEl('.js-notifContainer').removeClass('open');
    if (opts.closeOverlay) this.$navOverlay.removeClass('open');

    if (this.notifications) {
      const count = this.unreadNotifCount;
      if (this.unreadNotifCount) {
        const markAsRead = this.notifications.markNotifsAsRead();
        if (markAsRead) {
          this.unreadNotifCount = 0;
          markAsRead.fail(() => {
            this.unreadNotifCount = (this.unreadNotifCount || 0) + count;
          });
        }
      }

      this.notifications.reset();
    }
  }

  onClickNotificationLink() {
    this.closeNotifications();
  }

  onDocClick() {
    this.closeNotifications();
    this.closeNavMenu();
  }

  onFocusInAddressBar() {
    this.$addressBar.select();
  }

  onKeyupAddressBar(e) {
    if (e.which === 13) {
      const text = this.$addressBar.val().trim();
      this.$addressBar.val(text);

      const firstTerm = text.startsWith('ob://') ?
        text.slice(5)
          .split(' ')[0]
          .split('/')[0] :
        text.split(' ')[0]
          .split('/')[0];

      if (isMultihash(firstTerm)) {
        recordEvent('AddressBar_Input', { entry: 'multihash' });
        app.router.navigate(text.split(' ')[0], { trigger: true });
      } else if (firstTerm.charAt(0) === '@' && firstTerm.length > 1) {
        // a handle
        recordEvent('AddressBar_Input', { entry: 'handle' });
        app.router.navigate(text.split(' ')[0], { trigger: true });
      } else if (text.startsWith('ob://')) {
        // trying to show a specific page
        recordEvent('AddressBar_Input', { entry: 'ob://' });
        app.router.navigate(text.split(' ')[0], { trigger: true });
      } else {
        // searching term
        recordEvent('AddressBar_Input', { entry: 'searchTerm' });
        app.router.navigate(`search?q=${encodeURIComponent(text)}`, { trigger: true });
      }
    }
  }

  setAddressBar(text = '') {
    if (this.$addressBar) {
      this.addressBarText = text;
      this.$addressBar.val(text);
    }

    if (this.addressBarIndicators) this.addressBarIndicators.updateVisibility(text);
  }

  navSettingsClick() {
    // This is recorded as two events that belong to different metrics we're comparing.
    recordEvent('NavMenu_Click', { target: 'settings' });
    recordEvent('Settings_Open', { origin: 'navMenu' });
    launchSettingsModal();
  }

  navHelpClick() {
    recordEvent('NavMenu_Click', { target: 'help' });
    launchAboutModal({ initialTab: 'Help' });
    this.closeNavMenu();
  }

  navAboutClick() {
    recordEvent('NavMenu_Click', { target: 'about' });
    launchAboutModal({ initialTab: 'Story' });
    this.closeNavMenu();
  }

  navWalletClick() {
    recordEvent('NavClick', { target: 'walletOpen' });
    launchWallet();
  }

  navCreateListingClick() {
    // This is recorded as two events that belong to different metrics we're comparing.
    recordEvent('NavMenu_Click', { target: 'newListing' });
    recordEvent('Listing_New', { origin: 'navMenu' });
    const listingModel = new Listing({}, { guid: app.profile.id });

    launchEditListingModal({
      model: listingModel,
    });
  }

  remove() {
    if (this.unreadNotifCountFetch) this.unreadNotifCountFetch.abort();
    $(document).off('click', this.boundOnDocClick);
    super.remove();
  }

  render() {
    super.render();

    let connectedServer = getCurrentConnection();

    if (connectedServer && connectedServer.status !== 'disconnected') {
      connectedServer = connectedServer.server.toJSON();
    } else {
      connectedServer = null;
    }

    let showDiscoverCallout = false;

    if (connectedServer && !connectedServer.dismissedDiscoverCallout) {
      showDiscoverCallout = true;
    }

    loadTemplate('pageNav.html', (t) => {
      loadTemplate('walletIcon.svg', (walletIconTmpl) => {
        this.$el.html(t({
          addressBarText: this.addressBarText,
          connectedServer,
          testnet: app.serverConfig.testnet,
          walletIconTmpl,
          showDiscoverCallout,
          ...(app.profile && app.profile.toJSON() || {}),
        }));
      });
    });

    if (this.pageNavServersMenu) this.pageNavServersMenu.remove();
    this.pageNavServersMenu = new PageNavServersMenu({
      collection: app.serverConfigs,
    });
    this.$('.js-connManagementContainer').append(this.pageNavServersMenu.render().el);

    let initialAddressBarState = {};
    if (this.addressBarIndicators) {
      initialAddressBarState = this.addressBarIndicators.getState();
      this.addressBarIndicators.remove();
    }

    this.addressBarIndicators = this.createChild(AddressBarIndicators, {
      initialState: initialAddressBarState,
    });
    this.$('.js-addressBarIndicatorsContainer').html(this.addressBarIndicators.render().el);

    this.$addressBar = this.$('.js-addressBar');
    this.$navList = this.$('.js-navList');
    this.$navOverlay = this.$('.js-navOverlay');
    this.$connectedServerName = this.$('.js-connectedServerName');
    this.$connManagementContainer = this.$('.js-connManagementContainer');

    this.renderUnreadNotifCount();

    return this;
  }
}
