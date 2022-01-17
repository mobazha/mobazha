import app from '../../../app';
import '../../../lib/select2';
import { supportedWalletCurs } from '../../../data/walletCurrencies';
import { isJQPromise } from '../../../utils/object';
import loadTemplate from '../../../utils/loadTemplate';
import BaseView from '../../baseVw';
import CryptoTradingPair from '../../components/CryptoTradingPair';
import CryptoCurrencyTradeField from './CryptoCurrencyTradeField';

export default class extends BaseView {
  constructor(options = {}) {
    if (!options.model) {
      throw new Error('Please provide a Listing model.');
    }

    if (!isJQPromise(options.getCoinTypes)) {
      throw new Error('Please provide getCoinTypes as a jQuery promise.');
    }

    super(options);

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

      this.getCachedEl('.js-quantityCoinType')
        .text(selected);
    });

    this.tradeField.render();
    this.cryptoTradingPair.render();
  }

  className() {
    return 'cryptoCurrencyType padSmKids padStackAll';
  }

  events() {
    return {
      'change #editListingCoinType': 'onChangeCoinType',
      'change #editListingCryptoReceive': 'onChangeReceiveCur',
    };
  }

  onChangeCoinType(e) {
    this.getCachedEl('.js-quantityCoinType')
      .text(e.target.value);
    this.cryptoTradingPair.setState({
      toCur: e.target.value,
    });
  }

  onChangeReceiveCur(e) {
    this.cryptoTradingPair.setState({
      fromCur: e.target.value,
    });
  }

  get defaultFromCur() {
    return this.model.get('item').get('cryptoListingCurrencyCode') ||
      this.coinTypes ? this.coinTypes[0].code : '';
  }

  get tradeSelect2Opts() {
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
  }

  renderCryptoTradingPair() {

  }

  render() {
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

        this.getCachedEl('#editListingCryptoContractType').select2({
          minimumResultsForSearch: Infinity,
        });

        this.getCachedEl('#editListingCryptoReceive').select2(this.tradeSelect2Opts);

        this.tradeField.delegateEvents();
        this.getCachedEl('.js-cryptoCurrencyTradeContainer').html(this.tradeField.el);

        this.cryptoTradingPair.delegateEvents();
        this.getCachedEl('.js-cryptoTradingPairContainer').html(this.cryptoTradingPair.el);
      });
    });

    return this;
  }
}
