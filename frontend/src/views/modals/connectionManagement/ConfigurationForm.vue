<template>
  <div class="configurationForm" @click="onDocumentClick">
    <div class="contentBox padMd clrP clrBr clrSh3">
      <div class="flexVCent" style="height: 35px">
        <div class="col4"></div>
        <div class="col4">
          <h2 class="h3 clrT txUnl txCtr">{{ ob.title }}</h2>
        </div>
        <div class="col4"></div>
      </div>
      <hr :class="`clrBr ${!ob.builtIn ? 'rowLg' : ''}`" />

      <div v-if="ob.showConfigureTorMessage" class="border clrBr3 padMd flex torMessage">
        {{ ob.polyT('connectionManagement.configurationForm.configureTorMessage') }}
      </div>

      <div v-if="ob.showTorUnavailableMessage" class="border clrBrError clrTErr padMd flex torMessage">
        {{ ob.polyT('connectionManagement.configurationForm.torNotAvailableMessage') }}
      </div>

      <form :class="`padMdKids padStack ${formData.useTor ? 'useTor' : ''}`">
        <div class="padMdKids padStack pad0 js-standAloneSection" v-show="!ob.builtIn">
          <div class="flexRow">
            <div class="col3">
              <label for="serverConfigName" class="required">{{ ob.polyT('connectionManagement.configurationForm.name') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['name']" :errors="ob.errors['name']" />
              <input type="text" class="clrBr clrSh2 js-inputName" name="name" id="serverConfigName"
                v-focus
                v-model="formData.name"
                :placeholder="ob.polyT('connectionManagement.configurationForm.placeholderName')"
                data-field-standalone />
            </div>
          </div>
          <div class="flexRow">
            <div class="col3">
              <label for="serverConfigServerIp" class="required">{{ ob.polyT('connectionManagement.configurationForm.serverIp') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['serverIp']" :errors="ob.errors['serverIp']" />
              <input type="text" class="clrBr clrSh2" name="serverIp" id="serverConfigServerIp"
                :placeholder="ob.polyT('connectionManagement.configurationForm.placeholderServerIp')"
                v-model="formData.serverIp"
                data-field-standalone />
            </div>
          </div>
          <div class="flexRow">
            <div class="col3">
              <label for="serverConfigUsername" :class="`${ob.isRemote ? 'required' : ''} js-usernameLabel`">{{ ob.polyT('connectionManagement.configurationForm.username') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['username']" :errors="ob.errors['username']" />
              <input type="text" class="clrBr clrSh2" name="username" id="serverConfigUsername"
                v-model="formData.username"
                :placeholder="ob.polyT('connectionManagement.configurationForm.placeholderUsername')"
                data-field-standalone>
            </div>
          </div>
          <div class="flexRow">
            <div class="col3">
              <label for="serverConfigPassword" :class="`${ob.isRemote ? 'required' : ''} js-passwordLabel`">{{ ob.polyT('connectionManagement.configurationForm.password') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['password']" :errors="ob.errors['password']" />
              <input type="password" class="clrBr clrSh2" name="password" id="serverConfigPassword"
                v-model="formData.password"
                :placeholder="ob.polyT('connectionManagement.configurationForm.placeholderPassword')"
                data-field-standalone>
            </div>
          </div>
          <div class="flexRow">
            <div class="col3">
              <label>SSL</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['ssl']" :errors="ob.errors['ssl']" />
              <div class="btnStrip">
                <div class="btnRadio clrBr">
                  <input type="radio" name="SSL" value="true" id="serverConfigSSLOn"
                    data-field-standalone
                    v-model="formData.SSL">
                  <label for="serverConfigSSLOn">{{ ob.polyT('connectionManagement.configurationForm.sslOn') }}</label>
                </div>
                <div class="btnRadio clrBr">
                  <input type="radio" name="SSL" value="false" id="serverConfigSSLOff"
                    data-field-standalone
                    v-model="formData.SSL">
                  <label for="serverConfigSSLOff">{{ ob.polyT('connectionManagement.configurationForm.sslOff') }}</label>
                </div>
              </div>
            </div>
          </div>
          <div class="flexRow padBot0">
            <div class="col3">
              <label for="serverConfigPort" class="required">{{ ob.polyT('connectionManagement.configurationForm.port') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['port']" :errors="ob.errors['port']" />
              <input type="number" class="clrBr clrSh2" name="port" id="serverConfigPort"
                v-model="formData.port"
                :placeholder="ob.polyT('connectionManagement.configurationForm.placeholderPort')"
                data-field-standalone>
            </div>
          </div>
        </div>
        <div class="flexRow useTorCheckboxSection">
          <div class="col3">
            <label>{{ ob.polyT('connectionManagement.configurationForm.torLabel') }}</label>
          </div>
          <div class="col9">
            <FormError v-if="ob.errors['useTor']" :errors="ob.errors['useTor']" />
            <input type="checkbox" id="serverConfigUseTor" name="useTor"
              v-model="formData.useTor"
              data-field-standalone
              data-field-builtin />
            <label for="serverConfigUseTor">{{ ob.polyT('connectionManagement.configurationForm.useTor') }}</label>
          </div>
        </div>
        <div class="torDetails padMdKids padStack padBot0">
          <div v-if="!ob.builtIn"
            v-html='ob.polyT("connectionManagement.configurationForm.torServerWarning", { warning: `<span class="txB">${ob.polyT("connectionManagement.configurationForm.warning")}</span>`, })'>
          </div>
          <div class="flexRow">
            <div class="col3">
              <label class="required">{{ ob.polyT('connectionManagement.configurationForm.torProxyLabel') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['torProxy']" :errors="ob.errors['torProxy']" />
              <input type="text" class="clrBr clrSh2 required" name="torProxy" id="serverConfigTorProxy"
                v-model="formData.torProxy"
                :placeholder="ob.polyT('connectionManagement.configurationForm.torProxyPlaceholder')"
                data-field-standalone data-field-builtin>
            </div>
          </div>
          <div class="flexRow">
            <div class="col3">
              <label class="js-torPwLabel" :required="ob.isTorPwRequired">{{ ob.polyT('connectionManagement.configurationForm.torPwLabel') }}</label>
            </div>
            <div class="col9">
              <FormError v-if="ob.errors['torPassword']" :errors="ob.errors['torPassword']" />
              <input type="password" class="clrBr clrSh2" name="torPassword" id="serverConfigTorPw"
                v-model="formData.torPassword"
                :placeholder="ob.polyT('connectionManagement.configurationForm.torPwPlaceholder')"
                data-field-standalone data-field-builtin>
            </div>
          </div>
        </div>
      </form>

      <hr class="clrBr" />
      <div class="flexHRight flexVCent gutterHLg">
        <a @click="onCancelClick">{{ ob.polyT('connectionManagement.configurationForm.btnCancel') }}</a>
        <div class="posR">
          <a class="btn clrP clrBr clrSh2 " @click.stop.prevent="onSaveClick">{{ ob.polyT('connectionManagement.configurationForm.btnSave') }}</a>
          <div js-saveConfirmBox v-show="showSaveConfirmBox" class=" confirmBox saveConfirmBox arrowBoxBottom tx5 clrBr clrP clrT" @click.stop.prevent>
            <div class="tx3 txB rowSm">{{ ob.polyT('connectionManagement.configurationForm.saveConfirm.title') }}</div>
            <p>{{ ob.polyT('connectionManagement.configurationForm.saveConfirm.body') }}</p>
            <hr class="clrBr row" />
            <div class="flexHRight flexVCent gutterHLg buttonBar">
              <a @click="onClickSaveConfirmCancel">{{ ob.polyT('connectionManagement.configurationForm.saveConfirm.btnCancel') }}</a>
              <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph " @click="onSaveConfirmedClick">{{ ob.polyT('connectionManagement.configurationForm.saveConfirm.btnConfirm') }}</a>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import _ from 'underscore';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import { getCurrentConnection } from '../../../../backbone/utils/serverConnect';
import app from '../../../../backbone/app';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  watch: {
    'formData.serverIp'() {
      this.onChangeServerIp();
    },
  },
  data () {
    return {
      showSaveConfirmBox: false,

      formData: {
        name: '',
        serverIp: '',
        username: '',
        password: '',
        SSL: true,
        useTor: false,
        torProxy: '',
        torPassword: ''
      }
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
        ...this.model.toJSON(),
        errors: this.model.validationError || {},
        isRemote: !this.model.isLocalServer(),
        title: this.title,
        showConfigureTorMessage: this.showConfigureTorMessage,
        showTorUnavailableMessage: this.showTorUnavailableMessage,
        isTorPwRequired: this.model.isTorPwRequired(),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (!options.model) {
        throw new Error('Please provide a model.');
      }

      this._lastSavedAttrs = this.model.toJSON();
      this.formData = _.pick(this._lastSavedAttrs, _.keys(this.formData));

      const curConn = getCurrentConnection();
      this.showConfigureTorMessage = false;
      this.showTorUnavailableMessage = false;

      if (curConn && curConn.server && curConn.server.id === options.model.id) {
        if (curConn.reason === 'tor-not-configured') {
          this.showConfigureTorMessage = true;
        } else if (curConn.reason === 'tor-not-available') {
          this.showTorUnavailableMessage = true;
        }
      }

      this.title = this.model.isNew() ?
        app.polyglot.t('connectionManagement.configurationForm.tabName') :
        this.model.get('name');

      this.listenTo(this.model, 'change:name', () => {
        const newName = this.model.get('name');
        if (newName) this.title = newName;
      });
    },

    onDocumentClick () {
      this.showSaveConfirmBox = false;
    },

    onClickSaveConfirmCancel () {
      this.showSaveConfirmBox = false;
    },

    onCancelClick () {
      this.$emit('cancel', { view: this });
    },

    onSaveClick (e) {
      this.setModelFromForm();
      this.model.set({}, { validate: true });

      if (this.model.validationError) {
        return;
      }

      if (!this.model.isLocalServer() && !this.model.get('SSL')) {
        this.showSaveConfirmBox = true;
      } else {
        this.save();
      }
    },

    onSaveConfirmedClick () {
      this.save();
    },

    onChangeServerIp () {
      this.model.set(this.getFormData());

      if (!this.model.isLocalServer()) {
        // If you switched from a local to a remote IP, we'll default SSL
        // to on.
        if (this.model.isLocalServer(this.model.previousAttributes().serverIp)) {
          this.formData.SSL = true;
        }
      }
    },

    getFormData() {
      const builtIn = this.model.get('builtIn');
      if (builtIn) {
        return _.pick(this.formData, ['useTor', 'torProxy', 'torPassword']);
      }

      return this.formData;
    },

    setModelFromForm () {
      const builtIn = this.model.get('builtIn');
      const formData = this.getFormData();

      this.model.set({
        ...this.model.lastSyncedAttrs || {},
        ...formData,
        confirmedTor: this.model.get('confirmedTor') || formData.useTor ||
          this.showConfigureTorMessage,
        builtIn,
      });
    },

    /**
     * Save() assumes that you've previously called setModelFromForm to sync the model
     * from the UI.
     */
    save () {
      const save = this.model.save();

      if (save) {
        save.done(() => {
          this._lastSavedAttrs = this.model.toJSON();
          this.$emit('saved', { view: this });
        }).fail(() => {
          // since we're saving to localStorage this really shouldn't happen
          openSimpleMessage('Unable to save server configuration');
        });
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
