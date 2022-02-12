import $ from 'jquery';
import app from '../../../app';
import loadTemplate from '../../../utils/loadTemplate';
import baseVw from '../../baseVw';
import { unblock, isUnblocking, events as blockEvents } from '../../../utils/block';

export default class extends baseVw {
  constructor(options = {}) {
    const calcBlockedList = () => app.settings.get('blockedNodes')
      .filter(peerID => !isUnblocking(peerID));

    super({
      ...options,
      initialState: {
        blocked: calcBlockedList(),
      },
    });

    this.listenTo(blockEvents, 'unblocking unblockFail blocked',
      () => this.setState({ blocked: calcBlockedList() }));
  }

  className() {
    return 'settingsBlocked';
  }

  events() {
    return {
      'click .js-unblock': 'onClickUnblock',
    };
  }

  onClickUnblock(e) {
    const $peerIDEl = $(e.target).closest('[data-peerid]', this.el);
    const peerID = $peerIDEl.attr('data-peerid');

    if (!peerID) {
      throw new Error('Unable to unblock because the peerID data attribute is not set.');
    }

    unblock(peerID);
  }

  render() {
    super.render();
    loadTemplate('modals/settings/blocked.html', (t) => {
      this.$el.html(t({
        ...this.getState(),
      }));
    });

    return this;
  }
}
