import { events as blockEvents, unblock } from '../../utils/block';
import loadTemplate from '../../utils/loadTemplate';
import BaseModal from '../modals/BaseModal';

export default class extends BaseModal {
  constructor(options = {}) {
    const opts = {
      removeOnClose: true,
      showCloseButton: false,
      dismissOnEscPress: false,
      ...options,
    };

    if (typeof options.peerID !== 'string') {
      throw new Error('Please provide the peerID of the blocked user as a string.');
    }

    super(opts);
    this.options = opts;

    this.listenTo(blockEvents, 'unblocked unblocking', data => {
      if (data.peerIDs.includes(options.peerID)) this.close();
    });
  }

  className() {
    return `${super.className()} modalScrollPage modalMedium`;
  }

  events() {
    return {
      'click .js-cancel': 'onCancelClick',
      'click .js-btnUnblock': 'onUnblockClick',
      ...super.events(),
    };
  }

  onCancelClick() {
    this.trigger('canceled');
    this.close();
  }

  onUnblockClick() {
    unblock(this.options.peerID);
    this.close();
  }

  render() {
    loadTemplate('modals/blockedWarning.html', t => {
      this.$el.html(t({}));
      super.render();
    });

    return this;
  }
}
