<template>
  <div class="chatConvoMessages">
    <template v-for="model in collection">
      <ConvoMessage
        :options="messageOptions(model)"
        :bb="function() {
          return {
            model,
          };
        }"
        />
    </template>
  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../app';
import { checkValidParticipantObject } from './OrderDetail.js';

import ConvoMessage from './ConvoMessage.vue';

export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      _state: {
        showAvatar: true,
        showTimestampLine: true,
        showAsRead: false,
      }
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  unmounted() {
    this.timeAgoInterval.cancel();
  },
  computed: {
    ob () {

    },

  },
  methods: {
    loadData(options = {}) {
      if (!this.collection) {
        throw new Error('Please provide a chat messages collection.');
      }

      if (!options.$scrollContainer) {
        throw new Error('Please provide the DOM element that handles scrolling for this view.');
      }

      checkValidParticipantObject(options.buyer, 'buyer');
      checkValidParticipantObject(options.vendor, 'vendor');

      if (options.moderator) {
        checkValidParticipantObject(options.moderator, 'moderator');
      }

      this.baseInit(options);

      this.convoMessages = [];

      this.listenTo(app.profile.get('avatarHashes'), 'change', this.render);
    },

    markMessageAsRead(id) {
      if (!id) {
        throw new Error('Please provide an id.');
      }

      const message = this.collection.get(id);

      if (message) {
        const messageIndex = this.collection.indexOf(message);

        this.convoMessages[messageIndex]
          .setState({ showAsRead: true });

        // Only one message should be marked as read, so if there already was one,
        // we'll unmark it.
        if (this.messageMarkedAsRead) {
          const index = this.collection.indexOf(this.messageMarkedAsRead);

          if (index !== -1) {
            this.convoMessages[index].setState({ showAsRead: false });
          }
        }
      }
    },

    messageOptions(model) {
      if (!model) {
        throw new Error('Please provide a model.');
      }

      const initialState = {};

      let participant = this.buyer;
      initialState.role = 'buyer';
      let peerID = model.get('peerID');

      if (model.get('outgoing')) {
        initialState.avatarHashes = app.profile.get('avatarHashes').toJSON();
        peerID = app.profile.id;
      }

      if (peerID === this.vendor.id) {
        participant = this.vendor;
        initialState.role = 'vendor';
      } else if (this.moderator && peerID === this.moderator.id) {
        participant = this.moderator;
        initialState.role = 'moderator';
      }

      return {
        initialState: {
          ...initialState,
        },
      };

      // if (!model.get('outgoing')) {
      //   participant.getProfile().done(profileMd => {
      //     if (!convoMessage.isRemoved()) {

      //       convoMessage.setState({
      //         avatarHashes: profileMd.get('avatarHashes').toJSON(),
      //       });
      //     }
      //   });
      // }
    },

    createMessage(model, options = {}) {
      if (!model) {
        throw new Error('Please provide a model.');
      }

      const initialState = {};

      let participant = this.buyer;
      initialState.role = 'buyer';
      let peerID = model.get('peerID');

      if (model.get('outgoing')) {
        initialState.avatarHashes = app.profile.get('avatarHashes').toJSON();
        peerID = app.profile.id;
      }

      if (peerID === this.vendor.id) {
        participant = this.vendor;
        initialState.role = 'vendor';
      } else if (this.moderator && peerID === this.moderator.id) {
        participant = this.moderator;
        initialState.role = 'moderator';
      }


      const convoMessage = this.createChild(ConvoMessage, {
        ...options,
        model,
        initialState: {
          ...options.initialState,
          ...initialState,
        },
      });

      if (!model.get('outgoing')) {
        participant.getProfile().done(profileMd => {
          if (!convoMessage.isRemoved()) {
            convoMessage.setState({
              avatarHashes: profileMd.get('avatarHashes').toJSON(),
            });
          }
        });
      }

      this.convoMessages.push(convoMessage);

      return convoMessage;
    },

    render() {

      // We only want to mark the last 'read' message as read.
      this.messageMarkedAsRead = this.collection.slice()
        .reverse()
        .find(message => (message.get('read') && message.get('outgoing')));

      const lastReadIndex = this.collection.indexOf(this.messageMarkedAsRead);

      if (lastReadIndex !== -1) {
        this.convoMessages[lastReadIndex].setState({ showAsRead: true });
      }

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
