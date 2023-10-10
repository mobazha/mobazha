<template>
  <div class="socialAccounts gutterV">
    <div class="flexRow gutterH">
      <div class="col3">
        <label>{{ ob.polyT('settings.socialAccounts.accountLabel') }}</label>
        <div class="clrT2 txSm">{{ ob.polyT('settings.pageTab.helperAbout') }}</div>
      </div>
      <div class="col6">
        <div class="gutterV gutterVFlush js-socialWrapper"></div>
      </div>
    </div>
    <div class="flexRow gutterH">
      <div class="col3"></div>
      <div class="col6">
        <div class="flexCol">
          <button class="btnTxtOnly clrTEm txUnb txUHover" @click="onClickAddAccount" v-show="!ob.currentCount >= ob.max">
            {{ ob.polyT('settings.socialAccounts.add') }}
          </button>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import SocialAccount from './SocialAccount';
import SocialAccountMd from '../../../../backbone/models/profile/SocialAccount';


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
      accountViews: [],
      maxAccounts: 0,
    };
  },
  created () {
    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        currentCount: this.collection.length,
        max: this.maxAccounts,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!options.collection) {
        throw new Error('Please provide a collection.');
      }

      if (!options.maxAccounts) {
        throw new Error('Please provide a maximum number of accounts.');
      }

      this.baseInit(options);
      this.accountViews = [];
      this.maxAccounts = options.maxAccounts;

      this.listenTo(this.collection, 'add', (md) => {
        const view = this.createAccountView(md);
        this.accountViews.push(view);
        $('.js-socialWrapper').append(view.render().el);
        this.showLimit();
      });

      this.listenTo(this.collection, 'remove', (md, cl, removeOpts) => {
        this.accountViews.splice(removeOpts.index, 1)[0].remove();
        this.showLimit();
      });
    },

    get lastIndex () {
      return this.collection.length ? this.collection.length - 1 : 0;
    },

    addBlankAccount () {
      const notEmpty = !!this.collection.length;
      let name = 'type';
      const blank = notEmpty ? this.accountViews[this.lastIndex].firstBlankField : '';
      // if the current last account isn't completely filled in, don't add a new one
      if (!blank) {
        this.collection.add(new SocialAccountMd());
      } else {
        name = blank;
      }
      this.accountViews[this.lastIndex]
        .$(`input[name=${name}]`)
        .focus();
    },

    showLimit (show = this.accountViews.length >= this.maxAccounts) {
      if (show !== this._showLimit) {
        this._showLimit = show;
        $('.js-addAccount').toggleClass('hide', show);
      }
    },

    onClickAddAccount () {
      this.addBlankAccount();
    },

    setCollectionData () {
      this.accountViews.forEach((account) => {
        account.setModelData();
        // remove blank accounts
        if (!account.model.get('type') && !account.model.get('username')) {
          this.collection.remove(account.model);
        }
      });
    },

    createAccountView (model, options = {}) {
      const view = this.createChild(SocialAccount, {
        model,
        ...options,
      });

      this.listenTo(view, 'remove-click', () => {
        this.collection.remove(view.model);
        // if the last account is removed, replace it with a blank one
        if (this.collection.length === 0) {
          this.addBlankAccount();
        }
      });

      return view;
    },

    render () {
      this.accountViews.forEach((account) => account.remove());
      this.accountViews = [];

      const accountFrag = document.createDocumentFragment();

      this.collection.forEach((account) => {
        const view = this.createAccountView(account);
        this.accountViews.push(view);
        view.render().$el.appendTo(accountFrag);
      });

      $('.js-socialWrapper').append(accountFrag);

      // if the collection is empty on render, add a blank account to the form
      if (this.collection.length === 0) {
        this.collection.add(new SocialAccountMd());
      }

      return this;
    }
  }
}
</script>
<style lang="scss" scoped></style>
