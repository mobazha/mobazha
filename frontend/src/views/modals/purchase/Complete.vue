<template>
  <div class="complete">
    <div class="flexColRows gutterV">
      <div class="flexRow pad rowMd txCtr">
        <div class="col2"></div>
        <div class="col8">
          <div class="flexHCent row">
            <div class="discLg border clrE1 clrBrDec1">
              <i class="ion-checkmark clrTOnEmph"></i>
            </div>
          </div>
          <h4>{{ ob.polyT('purchase.completeSection.paymentSent') }}</h4>
          <p v-html='ob.polyT("purchase.completeSection.progressMessage", {
              link: `<a href="#transactions/purchases?orderID=${orderID}">${ob.polyT("purchase.completeSection.purchases")}</a>`
            })'></p>
          <p class="tx5b clrT2">
            {{ ob.polyT('purchase.completeSection.estimatedProcessing', { time: ob.processingTime }) }}
          </p>
        </div>
        <div class="col2"></div>
      </div>
      <div class="contentBox clrP clrBr padMd socialBtns">
        <h5>{{ ob.polyT('purchase.completeSection.share.title') }}</h5>
        <p>{{ ob.polyT('purchase.completeSection.share.body', { link: "<a class='clrTEm' href='https://mobazha.org'>https://mobazha.org</a>" }) }}</p>
        <div class="flexRow flexKidsExpand gutterH">
          <a class="btn btnThin clrP clrBr" :href="`https://twitter.com/intent/tweet/?text=${ob.polyT('purchase.completeSection.share.shareMsg')}&url=${shareURL}&hashtags=TradeFree,bitcoin&related=mobazha`">
            <span class="flexInline gutterHSm">
              <i class="ion-social-twitter twitterColor"></i><span>{{ ob.polyT('purchase.completeSection.share.postToTwitter') }}</span>
            </span>
          </a>
          <a class="btn btnThin clrP clrBr" :href="`https://www.facebook.com/sharer/sharer.php?u=${shareURL}`">
            <span class="flexInline gutterHSm">
              <i class="ion-social-facebook facebookColor"></i><span>{{ ob.polyT('purchase.completeSection.share.postToFacebook') }}</span>
            </span>
          </a>
          <a class="btn btnThin clrP clrBr" :href="`https://pinterest.com/pin/create/button/?url=${shareURL}`">
            <span class="flexInline gutterHSm">
              <i class="ion-social-pinterest pinterestColor"></i><span>{{ ob.polyT('purchase.completeSection.share.postToPinterest') }}</span>
            </span>
          </a>
          <a class="btn btnThin clrP clrBr" :href="`https://www.tumblr.com/share/link?url=${shareURL}&name=Mobazha`">
            <span class="flexInline gutterHSm">
              <i class="ion-social-tumblr tumblrColor"></i><span>{{ ob.polyT('purchase.completeSection.share.postToTumblr') }}</span>
            </span>
          </a>
        </div>
      </div>
      <h5>{{ ob.polyT('purchase.completeSection.vendorMessage') }}</h5>
      <div class="clrBr clrP flex gutterHSm message js-message">
        <div class="avatar clrBr2 clrSh1 disc" :style="ob.getAvatarBgImage(ob.ownProfile.avatarHashes)"></div>
        <div class="flexExpand">
          <textarea
            @keydown.enter.exact.prevent="keyDownMessageInput"
            class="clrP tx5"
            :placeholder="ob.polyT('purchase.completeSection.vendorMessagePlaceholder')"
            :maxlength="ob.maxMessageLength"
            rows="5"
            v-model="messageInput"
            ></textarea>
        </div>
      </div>
      <div class="flex gutterH">
        <div class="flexExpand tx5 clrTEm">
          <span class="js-messageSent" v-show="messageSentShow" v-html='ob.polyT("purchase.completeSection.vendorMessageSent", {
              name: ob.vendorName,
              orderLink: `<a class="txU" href="#transactions/purchases?orderID=${orderID}">${ob.polyT("purchase.completeSection.vendorMessageLink")}</a>`,
            })'>
          </span>
        </div>
        <button :class="`btn floR clrP clrBr clrSh2 js-send ${disabledSend? 'disabled' : ''}`" @click="sendMessageInput">{{ ob.polyT('purchase.completeSection.send') }}</button>
      </div>
    </div>

  </div>
</template>

<script>
import app from '../../../../backbone/app';
import ChatMessage from '../../../../backbone/models/chat/ChatMessage';
import Listing from '../../../../backbone/models/listing/Listing';

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
      shareURL: 'https://mobazha.org',
      messageInput: '',

      disabledSend: false,
      messageSentShow: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        displayCurrency: app.settings.get('localCurrency'),
        processingTime: this._listing.item.processingTime || app.polyglot.t('purchase.completeSection.noData'),
        maxMessageLength: ChatMessage.max.messageLength,
        ownProfile: app.profile.toJSON(),
        orderID: this.orderID,
        vendorName: this.options.vendor.name,
      };
    },
    vendorPeerID() {
      return this._listing.vendorID.peerID;
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.listing || !(this.listing instanceof Listing)) {
        throw new Error('Please provide a listing model');
      }

      if (!options.vendor) {
        throw new Error('Please provide a vendor object');
      }

      this.baseInit(options);
    },

    sendMessage (msg) {
      if (!msg) {
        throw new Error('Please provide a message to send.');
      }

      const chatMessage = new ChatMessage({
        peerID: this.vendorPeerID,
        orderID: this.orderID,
        message: msg,
      });

      const save = chatMessage.save();
      let messageSent = true;

      if (save) {
        this.disabledSend = true;
        this.messageSentShow = true;
      } else {
        // Developer error - this shouldn't happen.
        console.error('There was an error saving the chat message.');
        console.dir(chatMessage.validationError);
        messageSent = false;
      }

      return messageSent;
    },

    sendMessageInput () {
      const message = this.messageInput.trim();
      if (message) this.sendMessage(message);
      this.messageInput = ''
    },

    keyDownMessageInput () {
      // if the key pressed is not enter, do nothing

      this.disabledSend = !this.messageInput;
      this.messageSentShow = false;

      this.sendMessageInput();
    },

  }
}
</script>
<style lang="scss" scoped></style>
