<template>
  <div :class="`userCard ${isBlocked ? 'isBlocked' : ''}`">
    <div class="contentBox clrBr clrP clrSh2 <% if (ob.notFound) { %>disabled<% } %>">
      <div class="shortHeader pointer " @click="nameClick"
        :style="headerHash ? `background-image: url(${ob.getServerUrl(`ob/image/${headerHash}`)}), url('/imgs/defaultHeader.png')` : `background-image: url('/imgs/defaultHeader.png')`">
        <div class="blockedOverlay clrP flexCent tx5">
          <div>{{ ob.polyT('userShort.blockedUserOverlayText') }}</div>
        </div>
        <div class="userIconWrap">
          <a class="userIcon disc clrBr2 clrSh1"
            :style="avatarHash ? `background-image: url(${ob.getServerUrl(`ob/image/${avatarHash}`)}), url('/imgs/defaultAvatar.png')` : `background-image: url('/imgs/defaultAvatar.png')`">
          </a>
          <div class="blockedAvatarOverlay disc clrBr2 clrSh1 clrP clrT"><i class="ion-eye-disabled center"></i></div>
        </div>
        <template v-if="!ob.hideControls">
          <div class="userControls flexHRight gutterHSm">
            <template v-if="!ob.ownGuid">
              <template v-if="ob.moderator && ob.crypto.anySupportedByWallet(ob.moderatorInfo.acceptedCurrencies)">
                <ProcessingButton
                  :className="`iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-mod ${ob.ownMod ? 'active' : ''} ${processingMod ? 'processing' : ''}`"
                  @click.stop="modClick()"
                  btnText='<i class="ion-briefcase"></i>'
                  :data-tip="getModTip()" />
              </template>
              <ProcessingButton
                :className="`iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-follow ${ob.followedByYou ? 'active' : ''} ${processingFollow ? 'processing' : ''}`"
                @click.stop="followClick"
                btnText='<i class="ion-person-stalker"></i>'
                :data-tip="ob.getFollowTip(ob.followedByYou)" />
              <div class="js-blockBtnContainer">
                <BlockBtn :options="{ targetID: guid, initialState: { useIcon: true }, }" />
              </div>
            </template>
          </div>
        </template>
        <div ref="cardVerifiedMod" class="js-cardVerifiedMod">
          <VerifiedMod v-if="ob.verifiedMod" :options="verifiedModOptions()" />
        </div>
      </div>
      <div class="content">
        <template v-if="!ob.loading && !ob.notFound">
          <div class="contentTop">
            <div>
              <a class="flex snipKids gutterH rowTn userName " @click="nameClick">
                <div class="tx3 clrT"><strong>{{ ob.name }}</strong></div>
                <div class="clrT2">
                  {{ ob.handle ? `@${ob.handle}` : '' }}
                </div>
              </a>
            </div>
            <p class="clamp2 userDescription tx5" v-html="ob.shortDescription">
            </p>
          </div>
          <div class="flex gutterH contentBottom">
            <div class="flexExpand">
              <span class="clrT2 clamp tx5b"
                v-html="`${ob.parseEmojis('📍')} ${ob.location || ob.polyT('userPage.noLocation')}`"></span>
            </div>
            <a class="tx6 flexNoShrink ratingStrip" @click="ratingClick"
              v-html="ob.formatRating(ob.stats.averageRating, ob.stats.ratingCount)">
            </a>
          </div>
        </template>

        <template v-else-if="ob.loading">
          <div class="h3 clrT">{{ ob.polyT('userShort.userLoading') }}</div>
        </template>

        <template v-else-if="ob.notFound">
          <div class="h5 txUnb clrT" v-html="ob.polyT('userShort.userNotFound', { guid: ob.guid })"></div>
        </template>
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
import app from '../../../backbone/app';
import { followedByYou, followUnfollow } from '../../../backbone/utils/follow';
import Profile, { getCachedProfiles } from '../../../backbone/models/profile/Profile';
import { isBlocked, events as blockEvents } from '../../../backbone/utils/block';
import { openSimpleMessage } from '../../../backbone/views/modals/SimpleMessage';
import { getModeratorOptions } from '@/utils/verifiedMod';

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
  data () {
    return {
      isBlocked: false,

      guid: '',
      fetched: false,
      fetchedModel: undefined,

      loading: false,
      notFound: false,

      updateKey: 0,
      processingMod: false,
      processingFollow: false,

      followedByYou: false,

      settings: undefined,

      showModeratorDetails: false,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  unmounted() {
    if (this.profileFetch && this.profileFetch.abort) this.profileFetch.abort();
  },
  computed: {
    ob () {
      let access = this.updateKey;

      return {
        ...this.templateHelpers,
        loading: this.loading,
        notFound: this.notFound,
        guid: this.guid,
        ownGuid: this.guid === app.profile.id,
        followedByYou: this.followedByYou,
        ownMod: this.isOwnMod(),
        verifiedMod: app.verifiedMods.get(this.guid),
        getFollowTip: this.getFollowTip,
        ...this.options,
        ...((this.bb && this.model && this.model.toJSON()) || {}),
        ...((this.fetchedModel && this.fetchedModel.toJSON()) || {}),
      };
    },
    headerHash () {
      const ob = this.ob;
      return ob.headerHashes ? ob.isHiRez() ? ob.headerHashes.small : ob.headerHashes.tiny : '';
    },
    avatarHash () {
      const ob = this.ob;
      return ob.avatarHashes ? ob.isHiRez() ? ob.avatarHashes.small : ob.avatarHashes.tiny : '';
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (this.bb && this.model && this.model instanceof Profile) {
        this.guid = this.model.id;
        this.fetched = true;
      } else {
        this.guid = options.guid;
        this.fetched = false;
      }

      if (!this.guid) {
        if (_.has(this, 'model')) {
          throw new Error('The guid must be provided in the model.');
        } else {
          throw new Error('The guid must be provided in the options.');
        }
      }

      this.followedByYou = followedByYou(this.guid);

      this.loading = !this.fetched;
      this.settings = app.settings.clone();

      this.listenTo(this.settings, 'sync', () => {
        app.settings.set(this.settings.toJSON());
      });

      this.listenTo(app.settings, 'change:storeModerators', () => {
        this.updateKey += 1;
      });

      this.listenTo(app.ownFollowing, 'update', (cl, updateOpts) => {
        const updatedModels = updateOpts.changes.added.concat(updateOpts.changes.removed);

        if (updatedModels.filter((md) => md.id === this.guid).length) {
          this.followedByYou = followedByYou(this.guid);
          this.updateKey += 1;
        }
      });

      this.listenTo(blockEvents, 'blocked unblocked', (data) => {
        if (data.peerIDs.includes(this.guid)) {
          this.setBlockedClass();
        }
      });
    },

    isOwnMod() {
      return app.settings.ownMod(this.guid);
    },

    getModTip () {
      return this.isOwnMod()
        ? `${app.polyglot.t('userShort.tipModRemove')}`
        : `${app.polyglot.t('userShort.tipModAdd')}`;
    },

    getFollowTip (isFollowedByYou = this.followedByYou) {
      return isFollowedByYou
        ? `${app.polyglot.t('userShort.tipUnfollow')}`
        : `${app.polyglot.t('userShort.tipFollow')}`;
    },

    loadUser () {
      this.fetched = true;

      if (this.guid === app.profile.id) {
        // don't fetch this user's own profile, since we have it already
        this.profileFetch = $.Deferred().resolve(app.profile);
      } else {
        this.profileFetch = getCachedProfiles([this.guid])[0];
      }

      this.profileFetch.done((profile) => {
        this.loading = false;
        this.notFound = false;
        this.fetchedModel = profile;
        this.updateKey += 1;
      }).fail(() => {
        this.loading = false;
        this.notFound = true;

        this.updateKey += 1;
      });
    },

    attributes () {
      // make it possible to tab to this element
      return { tabIndex: 0 };
    },

    nameClick () {
      this.navToUser();
    },

    followClick () {
      const type = this.followedByYou ? 'unfollow' : 'follow';

      this.processingFollow = true;
      followUnfollow(this.guid, type)
        .always(() => (this.processingFollow = false));
    },

    modClick () {
      if (this.isOwnMod()) {
        // remove this user from the moderator list
        this.processingMod = true;
        this.saveModeratorList(false);
      } else {
        // show the moderator details modal
        this.showModeratorDetails = true;
      }
    },
    onAddAsModerator() {
      this.processingMod = true;
      this.saveModeratorList(true);
    },

    onClickImageHeader () {
      this.navToUser();
    },

    ratingClick () {
      this.navToUser('reputation');
    },

    navToUser (tab) {
      const route = `${this.guid}${tab ? `/${tab}` : ''}`;
      app.router.navigate(route, { trigger: true, });
    },

    saveModeratorList (add = false) {
      // clone the array, otherwise it is a reference
      let modList = _.clone(app.settings.get('storeModerators'));

      if (add && !this.isOwnMod()) {
        modList.push(this.guid);
      } else {
        modList = _.without(modList, this.guid);
      }

      const formData = { storeModerators: modList };
      this.settings.set(formData);

      if (!this.settings.validationError) {
        this.settings.save(formData, {
          attrs: formData,
          type: 'PUT',
        })
          .fail((...args) => {
            const errMsg = (args[0] && args[0].responseJSON && args[0].responseJSON.reason) || '';
            const phrase = add ? 'userShort.modAddError' : 'userShort.modRemoveError';
            openSimpleMessage(app.polyglot.t(phrase), errMsg);
          })
          .always(() => {
            this.processingMod = false;
          });
      }
    },

    setBlockedClass () {
      this.isBlocked = isBlocked(this.guid);
    },

    verifiedModOptions() {
      const verifiedMod = app.verifiedMods.get(this.guid);

      const createOptions = getModeratorOptions({
        model: verifiedMod,
      });

      return {
        ...createOptions,
        initialState: {
          ...createOptions.initialState,
          text: '',
        },
      };
    },

    render () {
      this.setBlockedClass();

      if (!this.fetched) this.loadUser();

      return this;
    },
  }
}
</script>
<style lang="scss" scoped></style>
