<template>
  <div class="userLoadingModal modal modalMedium modalScrollPage">
    <BaseModal :modalInfo="
      {
        dismissOnOverlayClick: false,
        dismissOnEscPress: false,
        showCloseButton: false,
        removeOnClose: false,
        removeOnRoute: true,
      }">
      <template v-slot:component>
        <section class="contentBox clrP clrBr clrSh3">
          <div class="padMd">
            <header>
              <template v-if="userAvatarHashes || userName">
                <div>
                  <div class="titleRow flexVCent gutterHSm">
                    <template v-if="userAvatarHashes">
                      <div class="discTn clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(userAvatarHashes)"></div>
                    </template>
                    <template v-if="userName">
                      <h2 class="h4 txUnl lineHeight1 clrT">{{ userName }}</h2>
                    </template>
                  </div>
                </div>
              </template>
            </header>
            <div class="txCtr">
              <div class="flexVCent flexInline gutterH row">
                <div class="discSm clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.ownAvatarHashes)"></div>
                <i :class="`ion-android-arrow-forward clrT2 lineHeight1 tx3 ${!isProcessing ? 'clrTErr' : ''}`"></i>
                <div
                  :class="`discSm clrBr2 clrSh1 flexNoShrink  ${!isProcessing ? 'disabled' : ''}`"
                  :style="ob.getAvatarBgImage(userAvatarHashes)"
                ></div>
              </div>
              <template v-if="isProcessing">
                <h1 class="h3 clrT">{{ ob.polyT('userPage.loading.connecting') }}</h1>
              </template>

              <template v-else>
                <h1 class="h3 clrTErr">{{ ob.polyT('userPage.loading.failedToConnect') }}</h1>
              </template>
              <div class="rowHg contentWrap">
                <div v-if="ob.contentHtml" v-html="ob.contentHtml"></div>

                <template v-else-if="contentText">
                  <p class="clrT2 tx5" v-html="contentText"></p>
                </template>
              </div>
              <p class="clrT2 tx6 rowSm">{{ ob.polyT('userPage.loading.socialHeading') }}</p>
              <div class="flexVCent flexInline gutterHSm socialIcons">
                <a href="https://twitter.com/mobazha" data-open-external>
                  <i class="ion-social-twitter twitterColor"></i>
                </a>
                <a href="https://www.facebook.com/MobazhaProject" data-open-external>
                  <i class="ion-social-facebook facebookColor"></i>
                </a>
                <a href="https://www.reddit.com/r/Mobazha/" data-open-external>
                  <i class="ion-social-reddit redditColor"></i>
                </a>
                <a href="https://github.com/Mobazha/mobazha" data-open-external>
                  <i class="ion-social-github githubColor"></i>
                </a>
                <a href="https://mobazha.org/slack/" data-open-external>
                  <img class="thumb non-ionic-icon" src="~@/../imgs/slack-icon.png" alt="Slack" />
                </a>
              </div>
            </div>
          </div>
          <div class="flexRow flexBtnWrapper">
            <a class="txCtr btnFlx flexExpand" @click="onClickCancel">{{ ob.polyT('userPage.loading.btnCancel') }}</a>
            <ProcessingButton
              :className="`btnFlx flexExpand clrP js-btnRetry ${isProcessing ? 'processing' : ''}`"
              :btnText="ob.polyT('userPage.loading.btnTryAgain')"
              @click="onClickBtnRetry"
            />
          </div>
        </section>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import app from '../../../backbone/app';

export default {
  props: {
    userName: {
      type: String,
      default: '',
    },
    userAvatarHashes: 
    {
      type: Object,
      default: undefined,
    },
    contentText: {
      type: String,
      default: '',
    },
    isProcessing: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      _state: {
        ownAvatarHashes: (app.profile && app.profile.get('avatarHashes').toJSON()) || undefined,
      }
    };
  },
  created() {
    this.loadData(this.options);
  },
  mounted() {},
  computed: {
    ob() {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        ...options,
        initialState: {
          ownAvatarHashes: (app.profile && app.profile.get('avatarHashes').toJSON()) || undefined,
          ...options.initialState,
        },
      };

      this.baseInit(opts);
    },

    onClickCancel() {
      this.$emit('clickCancel');
    },

    onClickBtnRetry() {
      this.$emit('clickRetry');
    },
  },
};
</script>
<style lang="scss" scoped></style>
