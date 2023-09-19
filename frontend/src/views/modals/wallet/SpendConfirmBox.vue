<template>
  <div class="spendConfirmBox centeredBelow" @click="onDocumentClick">
    <div v-if="ob.show">
      <div class="confirmBox sendConfirm arrowBoxTop clrBr clrP clrT clrSh1">
        <!-- // invisible heading for spacing purposes -->
        <div v-if="ob.fetchingFee" class="tx3 txB rowSm invisible">{{ ob.polyT('wallet.spendConfirmBox.heading') }}</div>
        <div v-else class="tx3 txB rowSm">{{ ob.polyT(ob.fetchError ? 'wallet.spendConfirmBox.errorHeading' : 'wallet.spendConfirmBox.heading') }}</div>

        <div class="posR padSm">
          <div v-if="ob.fetchingFee">
            <div class="txCtr">{{ ob.spinner({ className: 'spinnerMd' }) }}</div>
          </div>

          <div v-else-if="ob.fetchFailed">
            <div v-if="ob.fetchError === ob.ERROR_INSUFFICIENT_FUNDS">
              <p class="clrT2"> {{ ob.polyT('wallet.spendConfirmBox.bodyInsufficientFunds') }} </p>
            </div>
            <div v-else-if="ob.fetchError === ob.ERROR_DUST_AMOUNT">
              <p class="clrT2"> {{ ob.polyT('wallet.spendConfirmBox.bodyDustAmount') }} </p>
            </div>
            <div v-else>
              <p class="clrT2">{{ ob.polyT('wallet.spendConfirmBox.bodyGenericError', { err: ob.fetchError || '' }) }}</p>
              <a @click.stop="onClickRetry">{{ ob.polyT('wallet.spendConfirmBox.btnRetry') }}</a>
            </div>
          </div>

          <div v-else-if="ob.fee instanceof ob.bigNumber">
            <p class="clrT2">
              {{
                ob.polyT('wallet.spendConfirmBox.body', {
                  currencyPairing: ob.currencyMod.pairedCurrency(
                    ob.fee,
                    ob.coinType,
                    ob.displayCurrency
                  ),
                })
              }}
            </p>
          </div>
        </div>
        <hr class="clrBr row" />

        <div class="flexHRight flexVCent gutterHLg">
          <div v-if="ob.fetchingFee">
            <!-- // invisible button for spacing purposes -->
            <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph invisible">{{ ob.btnSendText }}</a>
          </div>

          <div v-else-if="!ob.fetchFailed">
            <a @click.stop="onClickCancel">{{ ob.polyT('wallet.spendConfirmBox.btnCancel') }}</a>
            <a class="btn clrBAttGrad clrBrDec1 clrTOnEmph " @click.stop="onClickSend">{{ ob.btnSendText }}</a>
          </div>

          <div v-else>
            <a @click.stop="onClickCancel">{{ ob.polyT('wallet.spendConfirmBox.btnClose') }}</a>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script>
import $ from 'jquery';
import app from '../../../../backbone/app';
import {
  ERROR_INSUFFICIENT_FUNDS,
  ERROR_DUST_AMOUNT,
} from '../../../../backbone/constants';
import { estimateFee } from '../../../../backbone/utils/fees';
import { validateNumberType } from '../../../../backbone/utils/number';
import {
  startPrefixedAjaxEvent,
  endPrefixedAjaxEvent,
  recordPrefixedEvent,
} from '../../../../backbone/utils/metrics';


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
    this.render();
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
        ERROR_INSUFFICIENT_FUNDS,
        ERROR_DUST_AMOUNT,
      };
    }
  },
  methods: {
    loadData (options = {}) {
      this._state = {
        show: false,
        fetchingFee: false,
        fetchFailed: false,
        fetchError: '',
        fee: false,
        coinType: '',
        displayCurrency: app.settings.get('localCurrency') || 'USD',
        btnSendText: app.polyglot.t('wallet.spendConfirmBox.btnConfirmSend'),
        ...options.initialState || {},
      };

      this.lastFetchFeeEstimateArgs = {};
      this.metricsOrigin = options.metricsOrigin;
    },

    onDocumentClick (e) {
      if (this.getState().show && !($.contains(this.el, e.target) || e.target === this.el)) {
        this.setState({ show: false });
      }
    },

    onClickSend () {
      this.$emit('clickSend');

      recordPrefixedEvent('ConfirmBoxSend', this.metricsOrigin);
    },

    onClickCancel () {
      this.setState({ show: false });
  
      recordPrefixedEvent('ConfirmBoxCancel', this.metricsOrigin);
    },

    onClickRetry () {
      const amount = this.lastFetchFeeEstimateArgs.amount;
      if (typeof amount === 'number') {
        this.fetchFeeEstimate(...this.lastFetchFeeEstimateArgs);
      }

      recordPrefixedEvent('ConfirmBoxRetry', this.metricsOrigin);
    },

  fetchFeeEstimate (
      amount,
      coinType = this.getState().coinType,
      feeLevel = app.localSettings.get('defaultTransactionFee')) {
      validateNumberType(amount, {
        fieldName: 'amount',
        isValidNumberOpts: {
          allowNumber: false,
          allowBigNumber: true,
          allowString: false,
        },
      });

      if (typeof coinType !== 'string' || !coinType) {
        throw new Error('Please provide the coinType as a string.');
      }

      this.lastFetchFeeEstimateArgs = {
        amount,
        coinType,
        feeLevel,
      };

      this.setState({
        fetchingFee: true,
        fetchError: '',
        fetchFailed: false,
        coinType,
      });

      startPrefixedAjaxEvent('ConfirmBoxEstimateFee', this.metricsOrigin);

      estimateFee(coinType, feeLevel, amount)
        .done(fee => {
          let state = {
            fee,
            fetchingFee: false,
          };

          if (
            app.walletBalances &&
            app.walletBalances.get(coinType) &&
            fee
              .plus(amount)
              .gt(
                app.walletBalances
                  .get(coinType)
                  .get('confirmed')
              )
          ) {
            state = {
              // The fetch didn't actually fail, but since the server allows unconfirmed spends and
              // we don't want to allow that, we'll pretend it failed and simulate the server
              // ERROR_INSUFFICIENT_FUNDS error.
              fetchFailed: true,
              fetchError: 'ERROR_INSUFFICIENT_FUNDS',
              ...state,
            };
            endPrefixedAjaxEvent('ConfirmBoxEstimateFee', this.metricsOrigin, {
              errors: 'ERROR_INSUFFICIENT_FUNDS',
            });
          } else {
            endPrefixedAjaxEvent('ConfirmBoxEstimateFee', this.metricsOrigin, {
              errors: 'none',
            });
          }

          this.setState(state);
        }).fail(err => {
          const fetchError = err || '';
          this.setState({
            fetchingFee: false,
            fetchFailed: true,
            fetchError,
          });

          endPrefixedAjaxEvent('ConfirmBoxEstimateFee', this.metricsOrigin, {
            errors: fetchError || 'unknown error',
          });
        });
    },
  }
}
</script>
<style lang="scss" scoped></style>
