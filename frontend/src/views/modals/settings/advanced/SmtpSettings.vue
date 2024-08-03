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
            <input type="radio" v-model="formData.notifications" value="true" id="smtpNotificationsOn" @input="onChangeShowSmtpNotifications" />
            <label for="smtpNotificationsOn">{{ ob.polyT('settings.on') }}</label>
          </div>
          <div class="btnRadio clrBr">
            <input type="radio" v-model="formData.notifications" value="false" id="smtpNotificationsOff" @input="onChangeShowSmtpNotifications" />
            <label for="smtpNotificationsOff">{{ ob.polyT('settings.off') }}</label>
          </div>
        </div>
      </div>
    </div>
    <div class="padMdKids padStack smtpSettings" v-show="!!(formData.notifications || smtpSettingsErrorFields.some(field => !!ob.errors[field]))">
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpServerAddress" class="required">{{ ob.polyT('settings.advancedTab.smtp.serverAddress')
          }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperServerAddress') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['serverAddress']" :errors="ob.errors['serverAddress']" />
          <input class="clrBr clrSh2" type="text" v-model="formData.serverAddress" id="smtpServerAddress"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderServerAddress')" />
          <div class="clrT2 txBase padSm" v-html='ob.polyT("settings.advancedTab.smtp.helperNewEmail", {
            helperGmailLink: `<a href="https://accounts.google.com/SignUp" class="clrTEm">${ob.polyT("settings.advancedTab.smtp.helperGmailLink")}</a>`
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
          <input class="clrBr clrSh2" type="text" v-model="formData.username" id="smtpUsername"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderUsername')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpPassword" class="required">{{ ob.polyT('settings.advancedTab.smtp.password') }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperPassword') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['password']" :errors="ob.errors['password']" />
          <input class="clrBr clrSh2" type="password" v-model="formData.password" id="smtpPassword"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderPassword')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpSenderEmail" class="required">{{ ob.polyT('settings.advancedTab.smtp.senderEmail') }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperSendFrom') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['senderEmail']" :errors="ob.errors['senderEmail']" />
          <input class="clrBr clrSh2" type="email" v-model="formData.senderEmail" id="smtpSenderEmail"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderSendFrom')" />
        </div>
      </div>
      <div class="flexRow gutterH">
        <div class="col3">
          <label for="smtpRecipientEmail" class="required">{{ ob.polyT('settings.advancedTab.smtp.recipientEmail') }}</label>
          <div class="clrT2 txSm">{{ ob.polyT('settings.advancedTab.smtp.helperSendTo') }}</div>
        </div>
        <div class="col6">
          <FormError v-if="ob.errors['recipientEmail']" :errors="ob.errors['recipientEmail']" />
          <input class="clrBr clrSh2" type="email" v-model="formData.recipientEmail" id="smtpRecipientEmail"
            :placeholder="ob.polyT('settings.advancedTab.smtp.placeholderSendTo')" />
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
                  <input class="clrBr clrP clrTEm" type="reset" @click="resetForm" :value="ob.polyT('settings.advancedTab.smtp.clearAction')" />
                </div>
                <div
                  :class="`flexNoShrink smtpTestButtonWrap js-smtpTestButtonWrap ${testingSmtp ? 'testInProgress' : ''}`">
                  <a class="btn clrP clrBr clrSh2  btnTest" @click="onClickTest">{{ ob.polyT('settings.advancedTab.smtp.testAction') }}</a>
                  <a class="btn clrP clrBr clrSh2  btnCancelTest" @click="onClickCancelTest">
                    <SpinnerSVG className="spinnerSm padSm" />
                    {{ ob.polyT('settings.advancedTab.smtp.testSmtpCancel') }}
                  </a>
                </div>
              </div>
            </div>
            <div>
              <div class="flexVCent js-testSmtpStatusContainer">
                <TestSmtpStatus :options="{ success, msg, }"/>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </form>
</template>

<script>
import _ from 'underscore';
import app from '../../../../../backbone/app';
import { myPost } from '../../../..//api/api';

import TestSmtpStatus from './TestSmtpStatus.vue';


export default {
  components: {
    TestSmtpStatus,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
      formData: {
        notifications: false,
        serverAddress: '',
        username: '',
        password: '',
        senderEmail: '',
        recipientEmail: '',
      },

      success: true,
      msg: '',

      testingSmtp: false,

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
  },
  unmounted () {
    if (this.testSmtpPost) this.testSmtpPost.abort();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: this.model.validationError || {},
      };
    }
  },
  methods: {
    loadData () {
      if (!this.model) {
        throw new Error('Please provide a SMTPSettings model.');
      }

      this.formData = _.pick(this.model.toJSON(), _.keys(this.formData));
    },

    onChangeShowSmtpNotifications () {
      this.setModelData();
    },

    resetForm () {
      if (this.testSmtpPost) this.testSmtpPost.abort();

      this.model.set({
        ..._.omit(this.model.defaults(), 'notifications'),
      });
      this.formData = _.pick(this.model.toJSON(), _.keys(this.formData));

      this.success = true;
      this.msg = '';
    },

    onClickTest () {
      if (this.testSmtpPost) this.testSmtpPost.abort();
      this.setModelData();
      this.model.set({}, { validate: true });

      this.msg = '';

      if (this.model.validationError) {
        return;
      }

      this.testingSmtp = true;
      this.testSmtpPost = myPost(app.getServerUrl('ob/testemailnotifications'), this.model.toJSON())
      .done(() => {
        this.success = true;
        this.msg = app.polyglot.t('settings.advancedTab.smtp.testSmtpSuccess');
      }).fail(xhr => {
        if (xhr.statusText === 'abort') return;
        const err = xhr.responseJSON && xhr.responseJSON.reason || '';
        const msg = err ?
          app.polyglot.t('settings.advancedTab.smtp.testSmtpFailWithError', { err }) :
          app.polyglot.t('settings.advancedTab.smtp.testSmtpFail');

        this.success = false;
        this.msg = msg;
      })
        .always(() => {
          this.testingSmtp = false;
        });
    },

    onClickCancelTest () {
      if (this.testSmtpPost) this.testSmtpPost.abort();

      this.testingSmtp = false;
    },

    // Sets the model based on the current data in the UI.
    setModelData (options = {}) {
      this.model.set(this.formData, options);
    },
  }
}
</script>
<style lang="scss" scoped></style>
