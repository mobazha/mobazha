<template>
  <div class="userPageHome">
    <div class="flexRow gutterH">
      <div class="col4 gutterVMd2">
        <div ref="userCard" class="userCard js-userCard">
          <UserCard
            :bb="function() {
              return {
                model,
              };
            }"
          />
        </div>
        <template v-if="ob.moderator && ob.moderatorInfo">
          <div class="contentBox padMd2 clrBr clrP clrSh2 js-moderatorInfo">
            <div class="flexVBot gutterH rowLg snipKids">
              <div class="tx4 flexExpand">
                <strong>{{ ob.polyT('userPage.moderator') }}</strong>
              </div>
              <div class="clrT2 tx5">
                <a class="link" @click="termsClick">
                  {{ ob.polyT('userPage.termsOfService') }}
                </a>
              </div>
            </div>
            <div class="tx2 clrT2 txCtr rowMd">
              {{ ob.polyT(`userPage.${ob.moderatorInfo.fee.feeType}`, { amount: feeAmount, percentage: ob.moderatorInfo.fee.percentage }) }}
            </div>
            <div class="flexHCent">
              <ProcessingButton
                v-show="!ownMod"
                :className="`btn clrP clrBr clrSh2 js-addModerator ${addingModerator ? 'processing' : ''}`"
                :btnText="ob.polyT('userPage.addModerator')"
                @click="addModeratorClick"
              />

              <ProcessingButton
                v-show="ownMod"
                :className="`btn clrP clrBr clrSh2 js-removeModerator ${removingModerator ? 'processing' : ''}`"
                :btnText="ob.polyT('userPage.removeModerator')"
                @click="removeModeratorClick"
              />
            </div>
          </div>
        </template>
        <div class="informationList listBox padMd2 clrBr clrP clrSh2">
          <div class="informationHeader">
            <strong>{{ ob.polyT('userPage.information') }}</strong>
          </div>
          <div class="listItem">
            <div class="flex">
              <span class="flexExpand">{{ ob.polyT('userPage.peerID') }}</span>
              <span ref="guidCopied" class="clrT hide js-guidCopied">{{ ob.polyT('copiedToClipboard') }}</span>
            </div>
            <div class="clrT2">
              <a class="clrT2" @click="guidClick(ob.peerID)" @mouseleave="guidLeave">{{ ob.peerID }}</a>
            </div>
          </div>
          <template v-if="ob.contactInfo">
            <template v-if="ob.contactInfo.website">
              <div class="listItem">
                <div>{{ ob.polyT('userPage.website') }}</div>
                <div>
                  <a class="clrT2" :href="ob.contactInfo.website.startsWith('http') ? ob.contactInfo.website : `http://${ob.contactInfo.website}`">{{
                    ob.contactInfo.website
                  }}</a>
                </div>
              </div>
            </template>
            <template v-if="ob.contactInfo.email">
              <div class="listItem">
                <div>{{ ob.polyT('userPage.email') }}</div>
                <div class="clrT2">{{ ob.contactInfo.email }}</div>
              </div>
            </template>
            <template v-for="account in ob.contactInfo.social" :key="account.type">
              <div class="listItem">
                <div>{{ account.type }}</div>
                <div class="clrT2">
                  <template v-if="ob.is.url(account.username)">
                    <a data-open-external class="clrT2" :href="account.username">{{ account.username }}</a>
                  </template>
                  <template v-else>{{ account.username }}</template>
                </div>
              </div>
            </template>
          </template>
        </div>
      </div>
      <div class="col8">
        <div class="box">
          <div class="aboutBox contentBox padMd clrBr clrP clrSh2">
            <div class="informationHeader row">
              <strong>{{ ob.polyT('userPage.about') }}</strong>
            </div>
            <template v-if="ob.about">
              <div v-html="ob.about" />
            </template>
            <template v-else>
              <span class="clrT2"><i>{{ ob.polyT('userPage.aboutEmpty') }}</i></span>
            </template>
          </div>
        </div>
      </div>
    </div>
    <Teleport to="#js-vueModal">
      <ModeratorDetails v-if="showModeratorDetails"
        :bb="() => {
          return {
            model,
          }}"
       @addAsModerator="onAddAsModerator"
       @close="showModeratorDetails = false"
      />
    </Teleport>
  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import { ipc } from '../../utils/ipcRenderer.js';
import app from '../../../backbone/app';
import { openSimpleMessage } from '../../../backbone/views/modals/SimpleMessage';

import ModeratorDetails from '@/views/modals/ModeratorDetails.vue';

export default {
  components: {
    ModeratorDetails,
  },
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data() {
    return {
      modsKey: 0,

      addingModerator: false,
      removingModerator: false,

      showModeratorDetails: false,
    };
  },
  created() {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted() {
  },
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        displayCurrency: app.settings.get('localCurrency'),
        ...this.model.toJSON(),
      };
    },
    feeAmount() {
      const ob = this.ob;
      if (ob.moderatorInfo.fee.fixedFee && ob.moderatorInfo.fee.fixedFee.amount) {
        return ob.currencyMod.convertAndFormatCurrency(
          ob.moderatorInfo.fee.fixedFee.amount,
          ob.moderatorInfo.fee.fixedFee.currency.code,
          ob.displayCurrency
        );
      }
      return 0;
    },
    ownMod() {
      let access = this.modsKey;
      return app.settings.ownMod(this.model.id);
    },
  },
  methods: {
    loadData(options = {}) {
      this.baseInit(options);

      this.ownPage = options.ownPage;
      this.settings = app.settings.clone();

      this.listenTo(this.settings, 'sync', () => {
        app.settings.set(this.settings.toJSON());
      });

      this.listenTo(app.settings, 'change:storeModerators', () => {
        this.modsKey += 1;
      });
    },

    termsClick() {
      // show the moderator details modal
      this.showModeratorDetails = true;
    },

    addModeratorClick() {
      // show the moderator details modal
      this.showModeratorDetails = true;
    },

    onAddAsModerator() {
      this.addingModerator = true;
      this.saveModeratorList(true);
    },

    removeModeratorClick() {
      this.removingModerator = true;
      this.saveModeratorList(false);
    },

    saveModeratorList(add = false) {
      // clone the array, otherwise it is a reference
      let modList = _.clone(app.settings.get('storeModerators'));

      if (add && !this.ownMod) {
        modList.push(this.model.id);
      } else {
        modList = _.without(modList, this.model.id);
      }

      const formData = { storeModerators: modList };
      this.settings.set(formData);

      if (!this.settings.validationError) {
        this.settings
          .save(formData, {
            attrs: formData,
            type: 'PUT',
          })
          .fail((...args) => {
            const errMsg = (args[0] && args[0].responseJSON && args[0].responseJSON.reason) || '';
            const phrase = add ? 'userPage.modAddError' : 'userPage.modRemoveError';
            openSimpleMessage(app.polyglot.t(phrase), { errMsg });
          })
          .always(() => {
            this.addingModerator = false;
            this.removingModerator = false;
          });
      }
    },

    guidClick(guid) {
      ipc.send('controller.system.writeToClipboard', guid);
      $(this.$refs.guidCopied).fadeIn(600);
    },

    guidLeave() {
      $(this.$refs.guidCopied).fadeOut(600);
    },
  },
};
</script>
<style lang="scss" scoped></style>
