<template>
  <div class="cryptoCurrencyType padSmKids padStackAll">
    <div class="flexRow titleWrap">
      <div class="col12">
        <div class="flexRow">
          <div class="flexExpand">
            <label for="editListingTitle">{{ ob.polyT('editListing.title') }}</label>
            <div class="js-cryptoTradingPairContainer"></div>
          </div>
          {{ ob.viewListingsT({ createMode: ob.createMode }) }}
        </div>
      </div>
    </div>
    <div class="flexRow gutterH">
      <div class="col6 simpleFlexCol">
        <label for="editListingCryptoContractType" class="required">{{ ob.polyT('editListing.type') }}</label>
        <template v-if="ob.metadata.contractType === 'CRYPTOCURRENCY' && ob.errors['metadata.contractType']">
          <FormError :errors="ob.errors['metadata.contractType']" />
        </template>
        <select id="editListingCryptoContractType" name="metadata.contractType" class="clrBr clrP clrSh2 marginTopAuto">
          <template v-for="(contractType, j) in ob.contractTypes" :key="j">
            <option :value="contractType.code" :selected="contractType.code === ob.metadata.contractType">{{
              contractType.name }}</option>
          </template>
        </select>
        <div class="clrT2 txSm helper">
          <template v-html="ob.polyT('editListing.cryptoCurrencyType.helperType', { count: `<b>
            ${ob.polyT('editListing.cryptoCurrencyType.helperTypeCount')}</b>`,
            })"></template>
        </div>
      </div>
    </div>
    <div class="flexRow gutterH">
      <div class="col6 simpleFlexCol">
        <label for="editListingCoinType" class="required">{{ ob.polyT('editListing.cryptoCurrencyType.coinType')
        }}</label>
        <FormError v-if="ob.errors['item.cryptoListingCurrencyCode']"
          :errors="ob.errors['item.cryptoListingCurrencyCode']" />
        <div class="js-cryptoCurrencyTradeContainer marginTopAuto"></div>
      </div>
      <div class="col6 simpleFlexCol">
        <label for="editListingCryptoQuantity" class="required">{{ ob.polyT('editListing.cryptoCurrencyType.quantity')
        }}</label>
        <FormError v-if="ob.errors['item.cryptoQuantity']" :errors="ob.errors['item.cryptoQuantity']" />
        <div class="posR">
          <input type="text" class="clrBr clrP clrSh2" name="item.cryptoQuantity" id="editListingCryptoQuantity"
            :value="ob.number.toStandardNotation(ob.item.cryptoQuantity)" placeholder="0.00" data-var-type="bignumber">
          <div class="cryptoQuantityCoinType clrT2 tx5 js-quantityCoinType">{{ ob.coinTypes ?
            ob.item.cryptoListingCurrencyCode || ob.coinTypes[0].code : '' }}</div>
        </div>
        <div class="clrT2 txSm helper">{{ ob.polyT('editListing.cryptoCurrencyType.helperQuantity') }}</div>
      </div>
    </div>
  </div>
  <div class="flexRow gutterH">
    <div class="col6 simpleFlexCol">
      <label for="editListingCryptoReceive" class="required">{{ ob.polyT('editListing.cryptoCurrencyType.lblReceive')
      }}</label>
      <div class="posR marginTopAuto">
        <template v-if="ob.errors['metadata.acceptedCurrencies'] && ob.metadata.contractType === 'CRYPTOCURRENCY'">
          <FormError :errors="ob.errors['metadata.acceptedCurrencies']" />
        </template>
        <select id="editListingCryptoReceive" @change="onChangeReceiveCur" name="metadata.acceptedCurrencies"
          class="clrBr clrP clrSh2 marginTopAuto">
          <template v-for="(coin, j) in ob.receiveCurs" :key="j">
            <option :value="coin.code" :selected="coin.code === ob.receiveCur">{{ coin.name }}</option>
          </template>
        </select>
        <div class="clrT2 txSm helper">{{ ob.polyT('editListing.cryptoCurrencyType.helperReceive') }}</div>
      </div>
    </div>
    <div class="col6 simpleFlexCol">
      <label for="editListingCryptoPriceModifier" class="required">{{
        ob.polyT('editListing.cryptoCurrencyType.priceModifier') }}</label>
      <FormError v-if="ob.errors['item.cryptoListingPriceModifier']"
        :errors="ob.errors['item.cryptoListingPriceModifier']" />
      <div class="posR marginTopAuto">
        <input type="text" class="clrBr clrP clrSh2" name="item.cryptoListingPriceModifier"
          id="editListingCryptoPriceModifier" :value="ob.item.cryptoListingPriceModifier"
          :placeholder="ob.polyT('editListing.cryptoCurrencyType.priceModifierPlaceholder')" data-var-type="number">
        <div class="cryptoPriceModifierPercentSymbol clrT2 tx5">%</div>
      </div>
      <div class="clrT2 txSm helper">{{ ob.polyT('editListing.cryptoCurrencyType.helperPriceModifier') }}</div>
    </div>
  </div>
</div></template>

<script>
import app from '../../../../backbone/app';
import { supportedWalletCurs } from '../../../../backbone/data/walletCurrencies';
import { isJQPromise } from '../../../../backbone/utils/object';
import loadTemplate from '../../../../backbone/utils/loadTemplate';
import CryptoTradingPair from '../../components/CryptoTradingPair';
import CryptoCurrencyTradeField from './CryptoCurrencyTradeField';


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
        contractTypes: this._model.metadata.contractTypesVerbose,
        coinTypes: this.coinTypes,
        receiveCurs: this.receiveCurs,
        errors: this.model.validationError || {},
        viewListingsT,
        ...this._model,
        receiveCur: this.options.getReceiveCur(),
      };
    }
  },
  methods: {
    loadData (options = {}) {
      if (!this.model) {
        throw new Error('Please provide a Listing model.');
      }

      if (!isJQPromise(options.getCoinTypes)) {
        throw new Error('Please provide getCoinTypes as a jQuery promise.');
      }

      this.baseInit(options);

      this.options = {
        getReceiveCur: () => this.model.get('metadata')
          .get('acceptedCurrencies')[0],
        ...options,
      };

      if (typeof this.options.getReceiveCur !== 'function') {
        throw new Error('If providing a getReceiveCur options, it must be a function.');
      }

      this.getCoinTypes = options.getCoinTypes;
      this.receiveCurs = supportedWalletCurs();
      const receiveCur = this.options.getReceiveCur();

      if (receiveCur && !this.receiveCurs.includes(receiveCur)) {
        // if the model has the receiving currency set to an unsupported cur,
        // we'll manually add that to the list of available options. Upon a
        // a save attempt, the user will be presented with an error prompting them
        // to select a valid currency.
        this.receiveCurs.push(receiveCur);
      }

      this.receiveCurs = this.receiveCurs.map(cur => ({
        code: cur,
        name: app.polyglot.t(`cryptoCurrencies.${cur}`, {
          _: cur,
        }),
      }));

      this.receiveCurs = this.receiveCurs.sort((a, b) => {
        if (a.name < b.name) return -1;
        if (a.name > b.name) return 1;
        return 0;
      });

      this.tradeField = this.createChild(CryptoCurrencyTradeField, {
        select2Opts: this.tradeSelect2Opts,
        initialState: {
          isFetching: this.getCoinTypes.state() === 'pending',
        },
      });

      // Initially we'll show this as 'invisible' for spacing purposes. A spinner will
      // show until the subsequent getCoinTypes() call returns.
      this.cryptoTradingPair = this.createChild(CryptoTradingPair, {
        className: 'cryptoTradingPairWrap row invisible',
        initialState: {
          tradingPairClass: 'cryptoTradingPairLg rowSm',
          exchangeRateClass: 'clrT2 tx6',
          // TODO
          // TODO
          // TODO - don't assume BTC, hard-code to the exchange rate reference coin
          fromCur: this.options.getReceiveCur() ||
            (this.receiveCurs[0] && this.receiveCurs[0].code) || 'BTC',
          toCur: 'BTC',
        },
      });

      this.getCoinTypes.done(curs => {
        const modelCur = this.model
          .get('item')
          .get('cryptoListingCurrencyCode');
        const selected = modelCur || curs[0].code;

        const currencies = [...curs];

        if (
          modelCur &&
          !currencies.find(cur => (cur.code === modelCur))
        ) {
          // The saved coin type is not in the list. Maybe there's no
          // exchange rate available. Maybe it's no longer on CMC. Anyhow,
          // we'll manually add it to the list otherwise the coin type just
          // defaults to the first coin and that's an odd experience to
          // just have your coinType swapped out on you like that.
          currencies.unshift({
            code: modelCur,
            name: app.polyglot.t(`cryptoCurrencies.${modelCur}`, {
              _: modelCur,
            }),
          });
        }

        this.coinTypes = currencies;

        this.tradeField.setState({
          curs: currencies,
          isFetching: false,
          selected,
        });

        this.cryptoTradingPair.$el.removeClass('invisible');
        this.cryptoTradingPair.setState({
          toCur: selected,
        });

        $('.js-quantityCoinType')
          .text(selected);
      });

      this.tradeField.render();
      this.cryptoTradingPair.render();
    },

    events () {
      return {
        'change #editListingCoinType': 'onChangeCoinType',
      };
    },

    onChangeCoinType (e) {
      $('.js-quantityCoinType')
        .text(e.target.value);
      this.cryptoTradingPair.setState({
        toCur: e.target.value,
      });
    },

    onChangeReceiveCur (e) {
      this.cryptoTradingPair.setState({
        fromCur: e.target.value,
      });
    }

  get defaultFromCur () {
      return this.model.get('item').get('cryptoListingCurrencyCode') ||
        this.coinTypes ? this.coinTypes[0].code : '';
    }

  get tradeSelect2Opts () {
      return {
        minimumResultsForSearch: 5,
        matcher: (params, data) => {
          if (!params.term || params.term.trim() === '') {
            return data;
          }

          const term = params.term
            .toUpperCase()
            .trim();

          if (
            data.text
              .toUpperCase()
              .includes(term) ||
            data.id.includes(term)
          ) {
            return data;
          }

          return null;
        },
      };
    },

    renderCryptoTradingPair () {

    },

    render () {
      super.render();

      loadTemplate('modals/editListing/viewListingLinks.html', viewListingsT => {
        loadTemplate('modals/editListing/cryptoCurrencyType.html', t => {
          this.$el.html(t({
            contractTypes: this.model.get('metadata').contractTypesVerbose,
            coinTypes: this.coinTypes,
            receiveCurs: this.receiveCurs,
            errors: this.model.validationError || {},
            viewListingsT,
            ...this.model.toJSON(),
            receiveCur: this.options.getReceiveCur(),
          }));

          $('#editListingCryptoContractType').select2({
            minimumResultsForSearch: Infinity,
          });

          $('#editListingCryptoReceive').select2(this.tradeSelect2Opts);

          this.tradeField.delegateEvents();
          $('.js-cryptoCurrencyTradeContainer').html(this.tradeField.el);

          this.cryptoTradingPair.delegateEvents();
          $('.js-cryptoTradingPairContainer').html(this.cryptoTradingPair.el);
        });
      });

      return this;
    }

  }
}
</script>
<style lang="scss" scoped></style>
