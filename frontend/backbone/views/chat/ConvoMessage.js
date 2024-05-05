import _ from 'underscore';
import moment from 'moment';
import { setTimeagoInterval } from '../../utils';
import app from '../../app';
import loadTemplate from '../../utils/loadTemplate';
import baseVw from '../baseVw';

export default class extends baseVw {
  constructor(options = {}) {
    if (!options.model) {
      throw new Error('Please provide a model.');
    }

    super(options);

    this._state = {
      showAvatar: true,
      showTimestampLine: true,
      showAsRead: false,
      ...options.initialState || {},
    };

    this.listenTo(this.model, 'change', () => this.render());

    this.timeAgoInterval = setTimeagoInterval(this.model.get('timestamp'), () => {
      const timeAgo = moment(this.model.get('timestamp')).fromNow();
      if (timeAgo !== this.renderedTimeAgo) this.render();
    });
  }

  className() {
    return 'convoMessage';
  }

  events() {
    return {
      'click .js-image': 'openImageModal',
      'click .js-image-close': 'closeImageModal',
    };
  }

  openImageModal() {
    this.getCachedEl('.js-imageModal').removeClass('hide');
  }

  closeImageModal() {
    this.getCachedEl('.js-imageModal').addClass('hide');
  }

  getState() {
    return this._state;
  }

  setState(state, replace = false, renderOnChange = true) {
    let newState;

    if (replace) {
      this._state = {};
    } else {
      newState = _.extend({}, this._state, state);
    }

    if (renderOnChange && !_.isEqual(this._state, newState)) {
      this._state = newState;
      this.render();
    }

    return this;
  }

  render() {
    this.renderedTimeAgo = moment(this.model.get('timestamp')).fromNow();

    const fileInChat = this.model.get('file');
    loadTemplate('chat/convoMessage.html', (t) => {
      this.$el.html(t({
        ...this.model.toJSON(),
        ...this.getState(),
        moment,
        message: this.model.get('message'),
        image: fileInChat && fileInChat.type === 'image'? app.getServerUrl(`ob/image/${fileInChat.hash}`) : null,
        renderedTimeAgo: this.renderedTimeAgo,
        ownGuid: app.profile.id,
      }));
    });

    return this;
  }
}
