<template>
  <form class="box padMdKids padStack clrP clrBr">
    <div class="flexRow">
      <div class="col3">
        <label>{{ ob.polyT('settings.advancedTab.smtp.notifications') }}</label>
        <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperNotifications') }}</div>
      </div>
      <div class="col6">
        <FormError v-if="ob.errors['notifications']" :errors="ob.errors['notifications']" />
        <div class="btnStrip">
          <div class="btnRadio clrBr">
            <input type="radio"
              name="notifications"
              value="true"
              id="smtpNotificationsOn"
              data-var-type="boolean"
              :checked="ob.notifications" />
            <label for="smtpNotificationsOn">{{ ob.polyT('settings.on') }}</label>
          </div>
          <div class="btnRadio clrBr">
            <input type="radio"
              name="notifications"
              value="false"
              id="smtpNotificationsOff"
              data-var-type="boolean"
              :checked="!ob.notifications" />
            <label for="smtpNotificationsOff">{{ ob.polyT('settings.off') }}</label>
          </div>
        </div>
      </div>
    </div>
    <div class="padMdKids padStack smtpSettings"
      v-show="!!(ob.notifications || smtpSettingsErrorFields.some(field => !!ob.errors[field]))">
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpServerAddress" class="required">{{ ob.polyT('settings.advancedTab.smtp.serverAddress')
          }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperServerAddress') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['serverAddress']" :errors="ob.errors['serverAddress']" />
          <input class="clrBr clrSh2"
            type="text"
            name="serverAddress"
            id="smtpServerAddress"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderServerAddress')"
            :value="ob.serverAddress" />
          <div class="clrT2 txBase padSm" v-html='ob.polyT("settings.advancedTab.smtp.helperNewEmail", {
            helperGmailLink: `<a
              href="https://accounts.google.com/SignUp"
              class="clrTEm">${ob.polyT("settings.advancedTab.smtp.helperGmailLink")}</a>`
          })'></div>
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpUsername" class="required">{{ ob.polyT('settings.advancedTab.smtp.username') }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperUsername') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['username']" :errors="ob.errors['username']" />
          <input class="clrBr clrSh2"
            type="text"
            name="username"
            id="smtpUsername"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderUsername')"
            :value="ob.username" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpPassword" class="required">{{ ob.polyT('settings.advancedTab.smtp.password') }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperPassword') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['password']" :errors="ob.errors['password']" />
          <input class="clrBr clrSh2"
            type="password"
            name="password"
            id="smtpPassword"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderPassword')"
            :value="ob.password" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpSenderEmail" class="required">{{ ob.polyT('settings.advancedTab.smtp.senderEmail') }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperSendFrom') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['senderEmail']" :errors="ob.errors['senderEmail']" />
          <input class="clrBr clrSh2"
            type="email"
            name="senderEmail"
            id="smtpSenderEmail"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderSendFrom')"
            :value="ob.senderEmail" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpRecipientEmail" class="required">{{ ob.polyT('settings.advancedTab.smtp.recipientEmail') }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperSendTo') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['recipientEmail']" :errors="ob.errors['recipientEmail']" />
          <input class="clrBr clrSh2"
            type="email"
            name="recipientEmail"
            id="smtpRecipientEmail"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderSendTo')"
            :value="ob.recipientEmail" />
        </div>
      </div>
      <div class="flexRow">
        <div class="col3">
        </div>
        <div class="col9">
          <div class="flex gutterH">
            <div class="flexNoShrink">
              <div class="flexVCent gutterH">
                <div class="flexNoShrink">
                  <input class="clrBr clrP clrTEm"
                    type="reset"
                    :value="ob.polyT('settings.advancedTab.smtp.clearAction')" />
                </div>
                <div
                  :class="`flexNoShrink smtpTestButtonWrap js-smtpTestButtonWrap ${ob.testingSmtp ? 'testInProgress' : ''}`">
                  <a class="btn clrP clrBr clrSh2  btnTest" @click="onClickTest">{{
                    ob.polyT('settings.advancedTab.smtp.testAction') }}</a>
                  <a class="btn clrP clrBr clrSh2  btnCancelTest" @click="onClickCancelTest">
                    <SpinnerSVG className="spinnerSm padSm" />
                    {{ ob.polyT('settings.advancedTab.smtp.testSmtpCancel') }}
                  </a>
                </div>
              </div>
            </div>
            <div>
              <div class="flexVCent js-testSmtpStatusContainer"></div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </form>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import app from '../../../../../backbone/app';
import TestSmtpStatus from './TestSmtpStatus';


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
      smtpSettingsErrorFields: [
        'serverAddress',
        'username',
        'password',
        'senderEmail',
        'recipientEmail',
      ],
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  unmounted () {
    if (this.testSmtpPost) this.testSmtpPost.abort();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: this.model.validationError || {},
        testingSmtp: this.testSmtpPost && this.testSmtpPost.state() === 'pending',
        ...this.model.toJSON(),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (!this.model) {
        throw new Error('Please provide a SMTPSettings model.');
      }
    },

    get events () {
      return {
        'change [name=notifications]': 'onChangeShowSmtpNotifications',
        'click input[type=reset]': 'resetForm',
      };
    },

    onChangeShowSmtpNotifications () {
      this.setModelData();
      this.render();
    },

    resetForm () {
      this.model.set({
        ..._.omit(this.model.defaults(), 'notifications'),
      });
      if (this.testSmtpPost) this.testSmtpPost.abort();
      if (this.testSmtpStatus) {
        this.testSmtpStatus.setState({
          isFetching: false,
          msg: '',
        });
      }
    },

    onClickTest () {
      if (this.testSmtpPost) this.testSmtpPost.abort();
      this.setModelData();
      this.model.set({}, { validate: true });
      this.testSmtpStatus.setState({ msg: '' });

      if (this.model.validationError) {
        this.render();
        return;
      }

      this.testSmtpPost = $.post({
        url: app.getServerUrl('ob/testemailnotifications'),
        data: JSON.stringify(this.model.toJSON()),
        dataType: 'json',
        contentType: 'application/json',
      }).done(() => {
        this.testSmtpStatus.setState({
          success: true,
          msg: app.polyglot.t('settings.advancedTab.smtp.testSmtpSuccess'),
        });
      }).fail(xhr => {
        if (xhr.statusText === 'abort') return;
        const err = xhr.responseJSON && xhr.responseJSON.reason || '';
        const msg = err ?
          app.polyglot.t('settings.advancedTab.smtp.testSmtpFailWithError', { err }) :
          app.polyglot.t('settings.advancedTab.smtp.testSmtpFail');

        this.testSmtpStatus.setState({
          success: false,
          msg,
        });
      })
        .always(() => {
          $('.js-smtpTestButtonWrap')
            .removeClass('testInProgress');
        });
    },

    onClickCancelTest () {
      if (this.testSmtpPost) this.testSmtpPost.abort();
      $('.js-smtpTestButtonWrap')
        .removeClass('testInProgress');
    },

    // Sets the model based on the current data in the UI.
    setModelData (options = {}) {
      this.model.set(this.getFormData(), options);
    },

    render () {
      const testSmtpStatusInitialState = {
        ...(this.testSmtpStatus && this.testSmtpStatus.getState() || {}),
      };

      if (this.testSmtpStatus) this.testSmtpStatus.remove();
      this.testSmtpStatus = this.createChild(TestSmtpStatus, {
        initialState: {
          ...testSmtpStatusInitialState,
        },
      });
      $('.js-testSmtpStatusContainer').html(this.testSmtpStatus.render().el);

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
