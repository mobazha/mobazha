<template>
  <div class="coinStats border clrP clrBr clrSh3">
    <!-- // the icon should be added after the text size class determination -->
    <div v-if="confirmedText" class="flexVCent gutterHTn">
      {{
        ob.polyT('cryptoAmountIconPairing', {
          amount: `<div>${confirmedText}</div>`,
          icon: ob.crypto.cryptoIcon({
            code: ob.cryptoCur,
            className: 'cryptoIcon18',
          })
        })
      }}
    </div>
    <div class="coinIcon">
      {{ ob.crypto.cryptoIcon({ code: ob.cryptoCur, className: '' }) }}
    </div>
    <div class="flex colWrap gutterH">
      <div :class="`${colClass} flexExpand`">
        <div class="flexCol flexHCent gutterVSm padSm">
          <div class="txB tx5">{{ ob.polyT('wallet.coinStats.balanceHeader') }}</div>
          <div :class="`clrTEm txB ${confirmedTextSizeClass}`">{{ confirmedText }}</div>
          <div class="clrT2 tx5b lineHeight1">{{ unconfirmedText }}</div>
        </div>
      </div>
      <div v-if="showDisplayCur">
        <div :class="`${colClass} flexExpand displayCurCol`">
          <div class="flexCol flexHCent gutterVSm padSm clrBr displayCurColContent">
            <div class="txB tx5">{{ ob.polyT('wallet.coinStats.valueInDisplayCur', { cur: ob.displayCur }) }}</div>
            <div :class="`clrTEm txB ${valueInDisplayCurSizeClass}`">
              <div v-if="ob.currencyMod.isFiatCur(ob.displayCur)">
                {{ ob.currencyMod.convertAndFormatCurrency(ob.confirmed, ob.cryptoCur, ob.displayCur) }}
              </div>

              <div v-else>
                <div class="flexVCent gutterHTn">
                  {{
                    ob.polyT('cryptoAmountIconPairing', {
                      amount:
                        `<div>${ob.currencyMod.convertAndFormatCurrency(ob.confirmed, ob.cryptoCur, ob.displayCur, {
                          includeCryptoCurIdentifier: false,
                        })}
                  </div>`,
                      icon: ob.crypto.cryptoIcon({
                        code: ob.displayCur,
                        className: 'cryptoIcon18',
                      })
                    })
                  }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div :class="`${colClass} flexExpand`">
        <div class="flexCol flexHCent gutterVSm padSm">
          <div class="txB tx5">{{ ob.polyT('wallet.coinStats.transactionsHeader') }}</div>
          <div class="clrTEm txB tx2">
            {{ typeof ob.transactionCount === 'number' ? ob.number.localizeNumber(ob.transactionCount) : '' }}
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import app from '../../../../backbone/app';

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

    this.loadData();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      };
    },
    isValidCryptoCur () {
      return typeof this.ob.cryptoCur === 'string' && this.ob.cryptoCur;
    },
    isValidDisplayCur () {
      return typeof this.ob.displayCur === 'string' && this.ob.displayCur;
    },
    cryptoCurHasExchangeRate () {
      return this.isValidCryptoCur && typeof ob.currencyMod.getExchangeRate(this.ob.cryptoCur) === 'number';
    },
    displayCurHasExchangeRate () {
      return this.isValidDisplayCur && typeof ob.currencyMod.getExchangeRate(this.ob.displayCur) === 'number';
    },
    showDisplayCur () {
      return this.isValidCryptoCur && this.isValidDisplayCur &&
        this.cryptoCurHasExchangeRate && this.displayCurHasExchangeRate &&
        this.ob.displayCur !== this.ob.cryptoCur;
    },
    colClass () {
      return showDisplayCur ? 'col4 statCol' : 'col6 statCol';
    },
    confirmedText () {
      return this.isValidCryptoCur ? ob.currencyMod.formatCurrency(this.ob.confirmed, this.ob.cryptoCur, { includeCryptoCurIdentifier: false }) : '';
    },
    confirmedTextSizeClass () {
      let confirmedTextSizeClass = 'tx2';
      confirmedTextSizeClass = confirmedText.length > 14 ? 'tx3' : confirmedTextSizeClass;
      confirmedTextSizeClass = confirmedText.length > 18 ? 'tx4' : confirmedTextSizeClass;
      return confirmedTextSizeClass;
    },
    unconfirmedText () {
      let unconfirmedText = this.ob.unconfirmed instanceof ob.bigNumber && this.isValidCryptoCur ?
        ob.currencyMod.formatCurrency(this.ob.unconfirmed, this.ob.cryptoCur, { useCryptoSymbol: false }) : '';
      unconfirmedText = unconfirmedText ? ob.polyT('wallet.coinStats.unconfirmedBalance', { amount: unconfirmedText }) : '';

      return unconfirmedText;
    },
    valueInDisplayCurSizeClass () {
      const valueInDisplayCur =
        ob.currencyMod.convertAndFormatCurrency(this.ob.confirmed, this.ob.cryptoCur, this.ob.displayCur, {
          includeCryptoCurIdentifier: false,
        });
      let valueInDisplayCurSizeClass = valueInDisplayCur.length > 14 ? 'tx3' : 'tx2';
      return valueInDisplayCur.length > 18 ? 'tx4' : valueInDisplayCurSizeClass;
    },
  },
  methods: {
    loadData(options = {}) {
      const opts = {
        initialState: {
          cryptoCur: '',
          displayCur: app && app.settings && app.settings.get('localCurrency') || 'USD',
          confirmed: undefined,
          unconfirmed: undefined,
          transactionCount: undefined,
          ...options.initialState,
        },
      };

      this.setState(opts.initialState || {});
    }
  }
}
</script>
<style lang="scss" scoped></style>
