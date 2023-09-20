<template>
  <div class="userLoadingModal modal modalMedium modalScrollPage">
    <BaseModal>
      <template v-slot:component>
        <section class="contentBox clrP clrBr clrSh3">
          <div class="padMd">
            <header>
              <div v-if="ob.userAvatarHashes || ob.userName">
                <div>
                  <div class="titleRow flexVCent gutterHSm">
                    <div v-if="ob.userAvatarHashes">
                      <div class=" discTn clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.userAvatarHashes)">
                      </div>
                    </div>
                    <div v-if="ob.userName">
                      <h2 class="h4 txUnl lineHeight1 clrT">{{ ob.userName }}</h2>
                    </div>
                  </div>
                </div>
              </div>
            </header>
            <div class="txCtr">
              <div class="flexVCent flexInline gutterH row">
                <div class="discSm clrBr2 clrSh1 flexNoShrink" :style="ob.getAvatarBgImage(ob.ownAvatarHashes)"></div>
                <i :class="`ion-android-arrow-forward clrT2 lineHeight1 tx3 ${!ob.isProcessing ? 'clrTErr' : ''}`"></i>
                <div :class="`discSm clrBr2 clrSh1 flexNoShrink  ${disabledToAvatar}`" :disabled="!ob.isProcessing"
                  :style="ob.getAvatarBgImage(ob.userAvatarHashes)"></div>
              </div>
              <div v-if="ob.isProcessing">
                <h1 class="h3 clrT">{{ ob.polyT('userPage.loading.connecting') }}</h1>
              </div>

              <div v-else>
                <h1 class="h3 clrTErr">{{ ob.polyT('userPage.loading.failedToConnect') }}</h1>
              </div>
              <div class="rowHg contentWrap">
                <div v-if="ob.contentHtml">
                  print(ob.contentHtml);
                </div>

                <div v-else-if="ob.contentText">
                  <p class="clrT2 tx5">{{ ob.contentText }}</p>
                </div>
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
                  <i class="thumb non-ionic-icon" style="background-image: url(../imgs/slack-icon.png)"></i>
                </a>
              </div>
            </div>
          </div>
          <div class="flexRow flexBtnWrapper">
            <a class="txCtr btnFlx flexExpand" @click="onClickCancel">{{ ob.polyT('userPage.loading.btnCancel') }}</a>
            <ProcessingButton
              :className="`btnFlx flexExpand clrP js-btnRetry ${ob.isProcessing ? 'processing' : ''}`"
              :btnText="ob.polyT('userPage.loading.btnTryAgain')"
              @click="onClickBtnRetry" />
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
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.$props.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
  },
  methods: {
    loadData (options = {}) {
      const opts = {
        ...options,
        dismissOnOverlayClick: false,
        dismissOnEscPress: false,
        showCloseButton: false,
        removeOnClose: false,
        removeOnRoute: true,
        initialState: {
          userName: '',
          contentText: '',
          isProcessing: false,
          ownAvatarHashes: (app.profile && app.profile.get('avatarHashes').toJSON()) || undefined,
          ...options.initialState,
        },
      };

      this.setState(opts.initialState || {});
    },

    onClickCancel () {
      this.$emit('clickCancel');
    },

    onClickBtnRetry () {
      this.$emit('clickRetry');
    },
  }
}
</script>
<style lang="scss" scoped></style>
