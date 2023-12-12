<template>
  <div class="socialAccounts gutterV">
    <div class="flexRow gutterH">
      <div class="col3">
        <label>{{ ob.polyT('settings.socialAccounts.accountLabel') }}</label>
        <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperAbout') }}</div>
      </div>
      <div class="col6">
        <div class="gutterV gutterVFlush js-socialWrapper">
          <template v-for="account in collection">
            <SocialAccount
              ref="accountViews"
              :bb="() => {
                return {
                  model: account,
                }
              }"
              @remove-click="onRemoveAccount(account)"
            />
          </template>
        </div>
      </div>
    </div>
    <div class="flexRow gutterH">
      <div class="col3"></div>
      <div class="col6">
        <div class="flexCol">
          <button class="btnTxtOnly clrTEm txUnb txUHover" @click="onClickAddAccount" v-show="collection.length < maxAccounts">
            {{ ob.polyT('settings.socialAccounts.add') }}
          </button>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import SocialAccountMd from '../../../../backbone/models/profile/SocialAccount';

import SocialAccount from './SocialAccount.vue';

export default {
  components: {
    SocialAccount,
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
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
  },
  methods: {
    loadData (options = {}) {
      if (!this.collection) {
        throw new Error('Please provide a collection.');
      }

      if (!options.maxAccounts) {
        throw new Error('Please provide a maximum number of accounts.');
      }

      this.baseInit(options);

      // if the collection is empty on render, add a blank account to the form
      if (this.collection.length === 0) {
        this.collection.add(new SocialAccountMd());
      }
    },

    addBlankAccount () {
      const lastIndex = this.collection.length ? this.collection.length - 1 : 0;

      const notEmpty = !!this.collection.length;
      let name = 'type';
      const blank = notEmpty ? this.$refs.accountViews[lastIndex].firstBlankField : '';
      // if the current last account isn't completely filled in, don't add a new one
      if (!blank) {
        this.collection.add(new SocialAccountMd());
      } else {
        name = blank;
      }
      this.$refs.accountViews[lastIndex].setFocus(name);
    },

    onClickAddAccount () {
      this.addBlankAccount();
    },

    setCollectionData () {
      (this.$refs.accountViews ?? []).forEach((account) => {
        account.setModelData();
        // remove blank accounts
        if (!account.model.get('type') && !account.model.get('username')) {
          this.collection.remove(account.model);
        }
      });
    },

    onRemoveAccount(md) {
      this.collection.remove(md);
      // if the last account is removed, replace it with a blank one
      if (this.collection.length === 0) {
        this.addBlankAccount();
      }
    },
  }
}
</script>
<style lang="scss" scoped></style>
