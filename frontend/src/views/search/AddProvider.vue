<template>
  <div class="addProvider" @click="onDocumentClick" v-if="showProvider">
    <div class="tx5 confirmBox arrowBoxCenteredTop padMd clrBr clrP clrSh1 clrT addBox js-addProvider" v-show="!ob.hideBox">
      <div class="flexCol gutterV txLft">
        <h3>{{ ob.polyT('search.addTitle') }}</h3>
        <FormError v-if="ob.errors[ob.urlType]" :errors="ob.errors[ob.urlType]" />
        <FormError v-if="ob.showExistsError" :errors="[ob.polyT('search.errors.existsError')]" />
        <input type="url" class="clrP clrBr clrSh2 js-addProviderInput" @keyup="onKeyUpAddProviderInput"
          :placeholder="ob.polyT(`${ob.usingTor ? 'search.addTorPlaceholder' : 'search.addPlaceholder'}`)">
        <div class="flexHRight flexVCent gutterH">
          <button class="btnTxtOnly barBtn clrT2 txUnb js-cancelBtn" @click="onClickCancel">{{ ob.polyT('search.cancel') }}</button>
          <button class="btn barBtn clrP clrBr clrSh2 js-addBtn" @click.stop="onClickAdd">{{ ob.polyT('search.addBtn') }}</button>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../backbone/app';
import { recordEvent } from '../../../backbone/utils/metrics';
import { curConnOnTor } from '../../../backbone/utils/serverConnect';
import { searchTypes } from '../../../backbone/utils/search';
import { openSimpleMessage } from '../../../backbone/views/modals/SimpleMessage';
import ProviderMd from '../../../backbone/models/search/SearchProvider';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      params: {},
      showProvider: true,
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        errors: {
          ...(this.model.validationError || {}),
        },
        ...this.params,
        ...this._state,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!searchTypes.includes(options.searchType)) {
        throw new Error('Please provide a valid search type.');
      }

      const opts = {
        searchType: '',
        initialState: {
          showExistsError: false,
          ...options.initialState,
        },
        ...options,
      };

      this.baseInit(opts);
      this.params = opts;

      this.model = new ProviderMd();
    },

    onDocumentClick (e) {
      if (!($.contains(this.el, e.target) || e.target === this.el)) {
        this.remove();
      }
    },

    save () {
      let URL = this.getCachedEl('.js-addProviderInput').val();
      // if the user doesn't type http:// or https://, add http:// for them
      if (!/^https?:\/\//i.test(URL)) {
        URL = `http://${URL}`;
      }

      /*
         If the exact same path as an existing provider is added, don't save. Note that if a base URL
         is added, like search.ob1.io, it won't be matched to provider URLs, since they include the
         full paths. This is to allow multiple providers on the same domain such as one at
         foo.com/shoeSearch and another at foo.com/hatSearch. This can be a little confusing, due to
         the self-healing mechanism where the endpoint returns search urls and those replace the urls
         the user enters, ie: entering "search.ob1.io" creates a provider that updates to use the
         returned listing endpoint, which is the same as the default OB1 search.
       */
      if (app.searchProviders.getProviderByURL(URL)) {
        this.setState({ showExistsError: true });
        return;
      }

      const opts = {};
      const urlType = `${curConnOnTor() ? 'tor' : ''}${this.params.searchType}`;
      opts[urlType] = URL;

      // pass the type of url to validate to the model
      this.model.set(opts, { validate: true, urlTypes: [urlType] });
      const modelErrors = this.model.validationError && this.model.validationError[urlType];
      if (!modelErrors) {
        const save = this.model.save(opts, { urlTypes: [urlType] });
        if (save) {
          // when saved successfully this view will be removed when the search is rerendered
          save.done(() => {
            recordEvent('Discover_AddProviderSaved', { errors: 'none', url: URL });
            app.searchProviders.add(this.model);
            this.trigger('newProviderSaved', this.model);
          })
            .fail(() => {
              // this is saved to local storage, errors shouldn't normally happen
              openSimpleMessage('This search provider could not be saved.');
            });
        }
      } else {
        recordEvent('Discover_AddProviderSaved', { errors: 'Invalid' });
        this.render();
      }
    },

    onKeyUpAddProviderInput (e) {
      if (e.which === 13) {
        this.save();
      }
    },

    onClickAdd (e) {
      this.save();
    },

    onClickCancel () {
      this.remove();
      recordEvent('Discover_AddProviderCancel');
    },

    remove () {
      this.showProvider = false;
    },

    render () {

      setTimeout(() => {
        this.getCachedEl('.js-addProviderInput').focus();
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
