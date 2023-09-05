<template>
  <div class="discussionTab clrP noMessages">
    <div class="typingIndicator tx5 noOverflow clrBr clrP clrT2 clrSh1">{{ typingIndicatorContent }}</div>
    <div class="convoMessagesWindow tx6 js-convoMessagesWindow">
      <SpinnerSVG />
      <div class="clrTErr messagesFetchError js-loadMessagesError" :hidden="!showLoadMessagesError">
        {{ ob.polyT('orderDetail.discussionTab.loadMessagesError', {
          retryLink: `<a class="" @click="onClickRetryLoadMessage">${ob.polyT('orderDetail.discussionTab.retryLink')}</a>`
        }) }}
      </div>
      <div class="js-convoMessagesContainer"></div>
    </div>

    <div class="clrBr clrP flex gutterHSm convoFooter js-convoFooter">
      <div class="avatar clrBr2 clrSh1 disc" :style="ob.getAvatarBgImage(app.profile.toJSON().avatarHashes)"></div>
      <div class="flexExpand">
        <textarea
          ref="inputMessage"
          class="clrP tx5"
          @keyup="onKeyUpMessageInput"
          @keydown="onKeyDownMessageInput"
          :placeholder="ob.polyT('chat.conversation.messageInputPlaceholder')"
          :maxlength="ChatMessage.max.messageLength"
          v-model="inputMessage"
          rows="1"></textarea>
        <div class="msgModUnableToChat clrT2">{{ ob.polyT('orderDetail.discussionTab.modCannotChat') }}</div>
      </div>
      <div>
        <button class="btn clrBAttGrad clrBrDec1 clrTOnEmph btnSend disabled" @click="onClickSend">{{
          ob.polyT('orderDetail.discussionTab.btnSend') }}</button>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import app from '../../../../backbone/app';
import { capitalize } from '../../../../backbone/utils/string';
import { getSocket } from '../../../../backbone/utils/serverConnect';
import GroupMessages from '../../../../backbone/collections/GroupMessages';
import ChatMessage from '../../../../backbone/models/chat/ChatMessage';
import { checkValidParticipantObject } from './OrderDetail.js';
import ConvoMessages from './ConvoMessages';


export default {
  mixins: [],
  data () {
    return {
      messagesPerPage: 20,
      typingExpires: 3000,

      inputMessage: '',
      typingIndicatorContent: '',
    };
  },
  created () {
    this.loadData(this.$props);
  },
  mounted () {
    this.render();

    this.typingIndicatorContent = this.getTypingIndicatorContent();
  },
  computed: {
    loadMessagesError () {
      let returnVal;

      if (this._loadMessagesError && this._loadMessagesError.length) {
        returnVal = this._loadMessagesError;
      } else {
        returnVal = (this._loadMessagesError = $('.js-loadMessagesError'));
      }

      return returnVal;
    },

    convoMessagesWindow () {
      return this._convoMessagesWindow ||
        (this._convoMessagesWindow = $('.js-convoMessagesWindow'));
    },

    btnSend () {
      return this._btnSend ||
        (this._btnSend = $('.js-btnSend'));
    },

    convoFooter () {
      return this._convoFooter ||
        (this._convoFooter = $('.js-convoFooter'));
    },

    /**
    * Returns the peerIDs that chat messages should be sent to.
    */
    sendToIds () {
      return this.getChatters()
        .filter(chatter => {
          // only include the moderator if the order is under
          // an active dispute
          let include = true;

          if (chatter.role === 'moderator' &&
            this.model.get('state') !== 'DISPUTED') {
            include = false;
          }

          return include;
        }).map(chatter => chatter.id);
    },
  },
  methods: {
    loadData (options = {}) {
      if (!options.orderID) {
        throw new Error('Please provide an orderID.');
      }

      if (!options.model) {
        throw new Error('Please provide an order / case model.');
      }

      if (typeof options.amActiveTab !== 'function') {
        throw new Error('Please provide an amActiveTab function that returns a boolean ' +
          'indicating whether Discussion is the active tab.');
      }

      checkValidParticipantObject(options.buyer, 'buyer');
      checkValidParticipantObject(options.vendor, 'vendor');

      if (options.moderator) {
        checkValidParticipantObject(options.moderator, 'moderator');
      }

      super(options);
      this.options = options;
      this.showLoadMessagesError = false;
      this.fetching = false;
      this.fetchedAllMessages = false;
      this.ignoreScroll = false;
      this.buyer = options.buyer;
      this.vendor = options.vendor;
      this.moderator = options.moderator;
      this.buyer.isTyping = false;
      this.vendor.isTyping = false;
      if (this.moderator) this.moderator.isTyping = false;

      this.messages = new GroupMessages([], { guid: this.model.id });
      this.listenTo(this.messages, 'request', this.onMessagesRequest);
      this.listenTo(this.messages, 'sync', this.onMessagesSync);
      this.listenTo(this.messages, 'update', this.onMessagesUpdate);
      this.listenTo(this.messages, 'error', this.onMessagesFetchError);
      this.fetchMessages();

      this.listenTo(this.model, 'change:state', () => {
        this.checkIfModCanChat();
        this.setMessageInputPlaceholder();
      });

      const socket = getSocket();

      if (socket) {
        this.listenTo(socket, 'message', this.onSocketMessage);
      }
    },



    onMessagesRequest (mdCl, xhr) {
      // Only interested in the collection sync (not any of its models).
      if (!(mdCl instanceof GroupMessages)) return;

      this.showLoadMessagesError = false;
      this.loadMessagesError.addClass('hide');
      this.$el.addClass('loadingMessages');
      this.fetching = true;

      xhr.always(() => (this.fetching = false));
    },

    onMessagesSync (mdCl, response) {
      // Only interested in the collection sync (not any of its models).
      if (!(mdCl instanceof GroupMessages)) return;

      this.showLoadMessagesError = false;
      this.loadMessagesError.addClass('hide');
      this.$el.removeClass('loadingMessages');

      if (response && !response.length) {
        this.fetchedAllMessages = true;
      }

      if (!this.firstSyncComplete) {
        this.firstSyncComplete = true;
        this.setScrollTop(this.convoMessagesWindow[0].scrollHeight);

        if (this.options.amActiveTab()) this.markConvoAsRead();
      }
    },

    onAttach () {
      if (this.firstSyncComplete) {
        this.markConvoAsRead();
      }
    },

    onMessagesFetchError () {
      this.showLoadMessagesError = true;
      this.loadMessagesError.removeClass('hide');
      this.$el.removeClass('loadingMessages');
    },

    onMessagesUpdate (cl, opts) {
      if (this.messages.length) {
        this.$el.removeClass('noMessages');
      }

      const prevTopModel = this.topRenderedMessageMd;

      if (!this.convoMessages) return;

      // As appropriate, update the scroll position.
      const prevScroll = {};

      prevScroll.height = this.convoMessagesWindow[0].scrollHeight;
      prevScroll.top = this.convoMessagesWindow[0].scrollTop;

      this.convoMessages.render();
      this.topRenderedMessageMd = this.messages.at(0);

      // Expecting either a new page of messages at the beginning of the collection or
      // a new single message at the end of the collection. In either of those scenarios
      // we'll adjust the scroll position as appopriate.

      if (opts.changes.added.length === 1) {
        // Single new message added.

        const newMessage = opts.changes.added[0];

        if (cl.indexOf(newMessage) === cl.length - 1) {
          // It's the last message.

          if (newMessage.get('outgoing')) {
            // It's our own message, so we'll auto scroll to the bottom.
            this.setScrollTop(this.convoMessagesWindow[0].scrollHeight);
          } else if (prevScroll.top >=
            prevScroll.height - this.convoMessagesWindow[0].clientHeight - 10) {
            // For an incoming message, if we were scrolled within 10px of the bottom at the
            // time the message came, we'll auto-scroll. Otherwise, we'll leave you where you were.
            this.setScrollTop(this.convoMessagesWindow[0].scrollHeight);
          }
        }
      } else if (opts.changes.added.length &&
        cl.indexOf(opts.changes.added[opts.changes.added.length - 1]) !==
        prevTopModel) {
        // New page of messages added up top. We'll adjust the scroll position so there is no
        // jump as they are added in.
        this.setScrollTop(prevScroll.top +
          (this.convoMessagesWindow[0].scrollHeight - prevScroll.height - 60));

        // the hardcode 60 is to account for the loading spinner that is going away
      }
    },

    onClickRetryLoadMessage () {
      this.fetchMessages(...this.lastFetchMessagesArgs);
    },

    onKeyUpMessageInput (e) {
      this.btnSend.toggleClass('disabled', !e.target.value);

      // Send an empty message to indicate "typing...", but no more than 1 every
      // second.
      if (!this.lastTypingSentAt || (Date.now() - this.lastTypingSentAt) >= 1000) {
        const typingMessage = new ChatMessage({
          peerIDs: this.sendToIds,
          orderID: this.model.id,
          message: '',
        });

        const saveTypingMessage = typingMessage.save();

        if (saveTypingMessage) {
          this.lastTypingSentAt = Date.now();
        } else {
          // Developer error - this shouldn't happen.
          console.error('There was an error saving the chat message.');
          console.dir(typingMessage.validationError);
        }
      }

      // Send actual chat message if the Enter key was pressed
      if (e.shiftKey || e.which !== 13) return;

      const message = e.target.value.trim();
      if (message) this.sendMessage(message);
      e.preventDefault();
    },

    onKeyDownMessageInput (e) {
      if (!e.shiftKey && e.which === 13) e.preventDefault();
    },

    onClickSend () {
      if (this.inputMessage) this.sendMessage(this.inputMessage);
    },

    onScroll (e) {
      if (this.ignoreScroll) {
        this.ignoreScroll = false;
        this.throttleScrollHandler();
        return;
      }

      if (this.fetching || this.fetchedAllMessages
        || this.showLoadMessagesError) {
        return;
      }

      // If we come close enough to the top, let's fetch a new page.
      if (e.target.scrollTop <= 100) {
        this.fetchMessages(this.messages.at(0).id);
      }
    },

    onSocketMessage (e) {
      if (e.jsonData.chatMessage && e.jsonData.chatMessage.orderID !== this.model.id) return;
      if (e.jsonData.messageTyping && e.jsonData.messageTyping.orderID !== this.model.id) return;
      if (e.jsonData.messageRead && e.jsonData.messageRead.orderID !== this.model.id) return;

      if (e.jsonData.chatMessage) {
        // incoming chat message
        const message = new ChatMessage({
          ...e.jsonData.chatMessage,
          outgoing: false,
        });

        this.messages.push(message);
        if (this.options.amActiveTab()) this.markConvoAsRead();

        // We'll consider them to be done typing if an actual message came
        // in. If they re-start typing, we'll get another socket message.
        const messageSender = this.getChatters()
          .find(chatter => chatter.id === e.jsonData.chatMessage.peerID);

        if (messageSender) {
          messageSender.isTyping = false;

          // if no one else is typing, we'll hide the indicator
          const typers = this.getChatters()
            .filter(chatter => chatter.isTyping);

          if (!typers.length) {
            this.hideTypingIndicator();
          } else {
            // update it so it doesn't show the message sender as typing
            this.setTypingIndicator();
          }
        }
      } else if (e.jsonData.messageTyping) {
        // Conversant is typing...
        this.setTyping(e.jsonData.messageTyping.peerID);
      } else if (e.jsonData.messageRead) {
        // Not using this for now since there are technical / UX complications for marking
        // a message as read when in a group chat (which user read it?).

        // Conversant read your message
        // if (this.convoMessages) {
        //   const model = this.messages.get(e.jsonData.messageRead.messageID);

        //   if (model) {
        //     model.set('read', true);
        //   }

        //   this.convoMessages.markMessageAsRead(e.jsonData.messageRead.messageID);
        // }
      }
    },

    setTyping (id) {
      if (!id) {
        throw new Error('Please provide an id.');
      }

      const chatters = this.getChatters();
      const typer = chatters.find(chatter => chatter.id === id);

      if (typer) {
        typer.isTyping = true;
        clearTimeout(typer.typingTimeout);
        typer.typingTimeout = setTimeout(
          () => (typer.isTyping = false),
          this.typingExpires);
        this.showTypingIndicator();
      }
    },

    showTypingIndicator () {
      clearTimeout(this.showTypingTimeout);
      this.setTypingIndicator();
      this.$el.addClass('isTyping');
      this.showTypingTimeout = setTimeout(
        () => (this.hideTypingIndicator()),
        this.typingExpires);
    },

    hideTypingIndicator () {
      clearTimeout(this.showTypingTimeout);
      this.$el.removeClass('isTyping');
    },


    sendMessage (msg) {
      if (!msg) {
        throw new Error('Please provide a message to send.');
      }

      this.lastTypingSentAt = null;

      const chatMessage = new ChatMessage({
        peerIDs: this.sendToIds,
        orderID: this.model.id,
        message: msg,
      });

      const save = chatMessage.save();
      let messageSent = true;

      if (save) {
        // At least for now, ignoring any server failures and optimistically adding the new
        // message to the UI. Odds are really low of server failure and repurcussions minimal.
        this.messages.push(chatMessage);
      } else {
        // Developer error - this shouldn't happen.
        console.error('There was an error saving the chat message.');
        console.dir(chatMessage.validationError);
        messageSent = false;
      }

      this.inputMessage = '';

      return messageSent;
    },

    fetchMessages (offsetID, limit = this.messagesPerPage) {
      const params = {
        limit,
        orderID: this.model.id,
      };

      this.lastFetchMessagesArgs = [offsetID, limit];
      if (offsetID) params.offsetID = offsetID;

      return this.messages.fetch({
        data: $.param(params),
        remove: false,
      });
    },

    setScrollTop (value, silent = true) {
      if (typeof value !== 'number') {
        throw new Error('Please provide a value as a number.');
      }

      if (this.convoMessagesWindow[0].scrollTop === value) return;

      if (silent) {
        // Unthrottling the scroll handler so that our ignoreScoll flag won't
        // be lumped in with a user triggered scroll. The scroll handler will
        // reset the flag and re-throttle the handler.
        this.unthrottleScrollHandler();
        this.ignoreScroll = true;
      }

      this.convoMessagesWindow[0].scrollTop = value;
    },

    unthrottleScrollHandler () {
      this.convoMessagesWindow.off('scroll', this.boundScrollHandler);
      this.boundScrollHandler = this.onScroll.bind(this);
      this.convoMessagesWindow.on('scroll', this.boundScrollHandler);
    },

    throttleScrollHandler () {
      this.convoMessagesWindow.off('scroll', this.boundScrollHandler);
      this.boundScrollHandler = _.throttle(this.onScroll, 100).bind(this);
      this.convoMessagesWindow.on('scroll', this.boundScrollHandler);
    },

    markConvoAsRead () {
      $.post({
        url: app.getServerUrl('ob/markchatasread'),
        data: JSON.stringify({
          orderID: this.model.id,
        }),
        dataType: 'json',
        contentType: 'application/json',
      });
      this.trigger('convoMarkedAsRead');
    },

    getChatters (includeSelf = false) {
      if (!this._chatters) {
        this._chatters = [
          {
            ...this.buyer,
            role: 'buyer',
          },
          {
            ...this.vendor,
            role: 'vendor',
          },
        ];

        if (this.moderator) {
          this._chatters.push({
            ...this.moderator,
            role: 'moderator',
          });
        }
      }

      let chatters = this._chatters;

      if (!includeSelf) {
        chatters = this._chatters.filter(chatter => chatter.id !== app.profile.id);
      }

      return chatters;
    },

    getTypingIndicatorContent () {
      let typingText = '';
      const typers = this.getChatters().filter(chatter => chatter.isTyping);
      const names = typers.map(typer =>
        app.polyglot.t(`orderDetail.discussionTab.role${capitalize(typer.role)}`));

      if (names.length === 1) {
        typingText = app.polyglot.t('orderDetail.discussionTab.isTyping', { userRole: names[0] });
      } else if (names.length === 2) {
        typingText = app.polyglot.t('orderDetail.discussionTab.areTyping', {
          userRole1: names[0],
          userRole2: names[1],
        });
      }

      return typingText;
    },

    setTypingIndicator () {
      this.typingIndicatorContent = this.getTypingIndicatorContent();
    },


    checkIfModCanChat () {
      if (this.moderator && this.moderator.id === app.profile.id &&
        this.model.get('state') === 'RESOLVED') {
        // If this is the moderator looking at the order and the mod has
        // already made a decision, the mod cannot send any more chat messages.
        this.convoFooter.addClass('preventModChat');
      }
    },

    setMessageInputPlaceholder () {
      if (this.moderator && this.moderator.id === app.profile.id) return;

      if (this.model.get('state') === 'DECIDED' ||
        this.model.get('state') === 'RESOLVED') {
        // If the mod has made a decision, indicator to the vendor / buyer
        // that they will no longer recieve new chat message.
        this.$refs.inputMessage.placeholder =
          app.polyglot.t('orderDetail.discussionTab.enterMessageNoMoreModPlaceholder');
      }
    },

    render () {
      this._loadMessagesError = null;
      this._convoMessagesWindow = null;
      this._btnSend = null;

      this._convoFooter = null;

      if (this.convoMessages) this.ConvoMessages.remove();
      this.convoMessages = new ConvoMessages({
        collection: this.messages,
        $scrollContainer: this.convoMessagesWindow,
        buyer: this.buyer,
        vendor: this.vendor,
        moderator: this.moderator,
      });
      $('.js-convoMessagesContainer').html(this.convoMessages.render().el);

      this.throttleScrollHandler();
      this.checkIfModCanChat();
      this.setMessageInputPlaceholder();

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
