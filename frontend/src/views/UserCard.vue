<template>
  <div class="userCard">
    <div class="contentBox clrBr clrP clrSh2 <% if (ob.notFound) { %>disabled<% } %>">
      <div class="shortHeader pointer " @click="nameClick"
        :style="headerHash ? `background-image: url(${ob.getServerUrl(`ob/image/${headerHash}`)}), url('../imgs/defaultHeader.png')` : `background-image: url('../imgs/defaultHeader.png')`">
        <div class="blockedOverlay clrP flexCent tx5">
          <div>{{ ob.polyT('userShort.blockedUserOverlayText') }}</div>
        </div>
        <div class="userIconWrap">
          <a class="userIcon disc clrBr2 clrSh1"
            :style="avatarHash ? `background-image: url(${ob.getServerUrl(`ob/image/${avatarHash}`)}), url('../imgs/defaultAvatar.png')` : `background-image: url('../imgs/defaultAvatar.png')`">
          </a>
          <div class="blockedAvatarOverlay disc clrBr2 clrSh1 clrP clrT"><i class="ion-eye-disabled center"></i></div>
        </div>
        <template v-if="!ob.hideControls">
          <div class="userControls flexHRight gutterHSm">
            <template v-if="!ob.ownGuid">
              <template v-if="ob.moderator && ob.crypto.anySupportedByWallet(ob.moderatorInfo.acceptedCurrencies)">
                <ProcessingButton
                  :className="`iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-mod ${ob.ownMod ? 'active' : ''}`"
                  @click.stop="modClick"
                  btnText='<i class="ion-briefcase"></i>'
                  :attrs="{ 'data-tip': ob.getModTip(ob.ownMod) }" />
              </template>
              <ProcessingButton
                :className="`iconBtnSm clrP clrBr toolTipNoWrap toolTipTop js-follow ${ob.followedByYou ? 'active' : ''}`"
                @click.stop="followClick"
                btnText='<i class="ion-person-stalker"></i>'
                :attrs="{ 'data-tip': ob.getFollowTip(ob.followedByYou) }" />
              <div class="js-blockBtnContainer"></div>
            </template>
          </div>
        </template>
        <div class="js-verifiedMod"></div>
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

  </div>
</template>

<script>
import $ from 'jquery';
import _ from 'underscore';
import loadTemplate from '../../backbone/utils/loadTemplate';
import app from '../../backbone/app';
import { followedByYou, followUnfollow } from '../../backbone/utils/follow';
import Profile, { getCachedProfiles } from '../../backbone/models/profile/Profile';
import { isBlocked, events as blockEvents } from '../../backbone/utils/block';
import { launchModeratorDetailsModal } from '../../backbone/utils/modalManager';
import { openSimpleMessage } from './modals/SimpleMessage';
import VerifiedMod, { getModeratorOptions } from './components/VerifiedMod';
import BlockedBtn from './components/BlockBtn';


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
      return {
        ...this.templateHelpers,
        loading: this.loading,
        notFound: this.notFound,
        guid: this.guid,
        ownGuid: this.ownGuid,
        followedByYou: this.followedByYou,
        ownMod: this.ownMod,
        getModTip: this.getModTip,
        getFollowTip: this.getFollowTip,
        ...this.options,
        ...((this.model && this.model.toJSON()) || {}),
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
    ownMod () {
      return app.settings.ownMod(this.guid);
    },
  },
  methods: {
    loadData (options = {}) {
      this.baseInit(options);

      if (this.model instanceof Profile) {
        this.guid = this.model.id;
        this.fetched = true;
      } else {
        this.guid = options.guid;
        this.fetched = false;
      }

      if (!this.guid) {
        if (this.model) {
          throw new Error('The guid must be provided in the model.');
        } else {
          throw new Error('The guid must be provided in the options.');
        }
      }

      this.ownGuid = this.guid === app.profile.id;
      this.followedByYou = followedByYou(this.guid);

      this.loading = !this.fetched;
      this.settings = app.settings.clone();

      this.listenTo(this.settings, 'sync', () => {
        app.settings.set(this.settings.toJSON());
      });

      this.listenTo(app.settings, 'change:storeModerators', () => {
        this.$modBtn.toggleClass('active', this.ownMod);
        this.$modBtn.attr('data-tip', this.getModTip());
      });

      this.listenTo(app.ownFollowing, 'update', (cl, updateOpts) => {
        const updatedModels = updateOpts.changes.added.concat(updateOpts.changes.removed);

        if (updatedModels.filter((md) => md.id === this.guid).length) {
          this.followedByYou = followedByYou(this.guid);
          this.$followBtn.toggleClass('active', this.followedByYou);
          this.$followBtn.attr('data-tip', this.getFollowTip());
        }
      });

      this.listenTo(blockEvents, 'blocked unblocked', (data) => {
        if (data.peerIDs.includes(this.guid)) {
          this.setBlockedClass();
        }
      });
    },

    getModTip (ownMod = this.ownMod) {
      return ownMod
        ? `${app.polyglot.t('userShort.tipModRemove')}`
        : `${app.polyglot.t('userShort.tipModAdd')}`;
    },

    getFollowTip (isFollowedByYou = this.followedByYou) {
      return isFollowedByYou
        ? `${app.polyglot.t('userShort.tipUnfollow')}`
        : `${app.polyglot.t('userShort.tipFollow')}`;
    },

    loadUser (guid = this.guid) {
      this.fetched = true;

      if (guid === app.profile.id) {
        // don't fetch this user's own profile, since we have it already
        this.profileFetch = $.Deferred().resolve(app.profile);
      } else {
        this.profileFetch = getCachedProfiles([guid])[0];
      }

      this.profileFetch.done((profile) => {
        if (this.isRemoved()) return;
        this.loading = false;
        this.notFound = false;
        this.model = profile;
        this.render();
      }).fail(() => {
        if (this.isRemoved()) return;
        this.loading = false;
        this.notFound = true;
        this.render();
      });
    },

    attributes () {
      // make it possible to tab to this element
      return { tabIndex: 0 };
    },

    events () {
      return {
        'click .js-follow': 'followClick',
        'click .js-mod': 'modClick',
      };
    },

    nameClick () {
      this.navToUser();
    },

    followClick (e) {
      const type = this.followedByYou ? 'unfollow' : 'follow';

      this.$followBtn.addClass('processing');
      followUnfollow(this.guid, type)
        .always(() => (this.$followBtn.removeClass('processing')));
    },

    modClick (e) {
      if (this.ownMod) {
        // remove this user from the moderator list
        this.$modBtn.addClass('processing');
        this.saveModeratorList(false);
      } else {
        // show the moderator details modal
        const modModal = launchModeratorDetailsModal({ model: this.model });
        this.listenTo(modModal, 'addAsModerator', () => {
          this.$modBtn.addClass('processing');
          this.saveModeratorList(true);
        });
      }
    },

    onClickImageHeader () {
      this.navToUser();
    },

    ratingClick () {
      this.navToUser('reputation');
    },

    navToUser (tab) {
      const route = `${this.guid}${tab ? `/${tab}` : ''}`;
      app.router.navigate(route, {
        trigger: true,
      });
    },

    saveModeratorList (add = false) {
      // clone the array, otherwise it is a reference
      let modList = _.clone(app.settings.get('storeModerators'));

      if (add && !this.ownMod) {
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
            this.$modBtn.removeClass('processing');
          });
      }
    },

    setBlockedClass () {
      this.$el.toggleClass('isBlocked', isBlocked(this.guid));
    },

    get $followBtn () {
      if (!this._$followBtn) {
        this._$followBtn = $('.js-follow');
      }
      return this._$followBtn;
    },

    get $modBtn () {
      if (!this._$modBtn) {
        this._$modBtn = $('.js-mod');
      }
      return this._$modBtn;
    },

    render () {
      loadTemplate('userCard.html', (t) => {
        this.$el.html(t({
          loading: this.loading,
          notFound: this.notFound,
          guid: this.guid,
          ownGuid: this.ownGuid,
          followedByYou: this.followedByYou,
          ownMod: this.ownMod,
          getModTip: this.getModTip,
          getFollowTip: this.getFollowTip,
          ...this.options,
          ...((this.model && this.model.toJSON()) || {}),
        }));

        this._$followBtn = null;
        this._$modBtn = null;

        if (this.guid !== app.profile.id) {
          $('.js-blockBtnContainer')
            .html(
              new BlockedBtn({
                targetId: this.guid,
                initialState: {
                  useIcon: true,
                },
              }).render().el,
            );
        }

        this.setBlockedClass();

        if (!this.fetched) this.loadUser();
        /* the view should be rendered when it is created and before it has data, so it can occupy
         space in the DOM while the data is being fetched. */

        if (this.verifiedMod) this.verifiedMod.remove();

        const verifiedMod = app.verifiedMods.get(this.guid);
        const createOptions = getModeratorOptions({
          model: verifiedMod,
        });
        if (verifiedMod && this.model && this.model.isModerator) {
          this.verifiedMod = this.createChild(VerifiedMod, {
            ...createOptions,
            initialState: {
              ...createOptions.initialState,
              text: '',
            },
          });
          $('.js-verifiedMod').append(this.verifiedMod.render().el);
        }
      });

      return this;
    },
  }
}
</script>
<style lang="scss" scoped></style>
