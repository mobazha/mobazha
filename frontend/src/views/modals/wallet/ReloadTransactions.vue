<template>
  <div class="flexVCent gutterH">
    <div class="posR tx5b txU">
      <template v-if="!ob.isResyncAvailable">
        <span class="toolTip" :data-tip="ob.polyT('wallet.reloadTransactionsWidget.syncUnavailable')"
          style="text-align: left">
          <a class=" disabled margRSm" @click="onClickResync">{{ ob.polyT('wallet.reloadTransactionsWidget.resyncBtn')
          }}</a><span class="ion-android-warning clrTErr"></span>
        </span>
      </template>

      <template v-else-if="ob.isSyncing">
        <a class="invisible">{{ ob.polyT('wallet.reloadTransactionsWidget.resyncBtn') }}</a>
        <SpinnerSVG className="center spinnerSm" />
      </template>

      <template v-else>
        <a class="" @click="onClickResync">{{ ob.polyT('wallet.reloadTransactionsWidget.resyncBtn') }}</a>
      </template>
    </div>
  </div>
</template>

<script>
import app from '../../../../backbone/app';
import { openSimpleMessage } from '../../../../backbone/views/modals/SimpleMessage';
import resyncBlockchain, {
  isResyncAvailable,
  isResyncingBlockchain,
  events as resyncEvents,
} from '../../../../backbone/utils/resyncBlockchain';
import { ensureMainnetCode } from '../../../../backbone/data/walletCurrencies';
import { recordEvent } from '../../../../backbone/utils/metrics';


export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
  },
  data () {
    return {
      _state: {
        coinType: '',
        isSyncing: false,
        isResyncAvailable: false,
      }
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
  },
  computed: {
    ob () {
      return {
        ...this.templateHelpers,
        ...this._state,
      }
    },
  },
  methods: {
    loadData (options = {}) {
      if (!options.initialState ||
        (typeof options.initialState.coinType !== 'string' &&
          !options.initialState.coinType)) {
        throw new Error('Please provide a coinType in the initial state');
      }

      const opts = {
        ...options,
        initialState: {
          isSyncing: isResyncingBlockchain(options.initialState.coinType),
          isResyncAvailable: isResyncAvailable(options.initialState.coinType),
          ...options.initialState || {},
        },
      };

      this.baseInit(opts);

      this.listenTo(resyncEvents, 'resyncing', e => {
        if (e.coinType === this.getState().coinType) {
          this.setState({
            isSyncing: true,
          });
        }
      });

      this.listenTo(resyncEvents, 'resyncComplete', e => {
        if (e.coinType === this.getState().coinType) {
          this.setState({
            isSyncing: false,
          });
        }
      });

      this.listenTo(resyncEvents, 'resyncFail', e => {
        if (e.coinType === this.getState().coinType) {
          this.setState({ isSyncing: false });
        }
      });

      this.listenTo(resyncEvents, 'changeResyncAvailable', e => {
        if (e.coinType === this.getState().coinType) {
          this.setState({ isResyncAvailable: e.available });
        }
      });
    },

    onClickResync () {
      const coinType = this.getState().coinType;
      recordEvent('Wallet_Resync');
      resyncBlockchain(coinType)
        .done(() => {
          openSimpleMessage(
            app.polyglot.t('wallet.reloadTransactionsWidget.resyncCompleteTitle', {
              cur: ensureMainnetCode(coinType),
            }),
            app.polyglot.t('wallet.reloadTransactionsWidget.resyncComplete')
          );
        });
    },

    setState(state = {}, options = {}) {
      const curState = this.getState();
      if (state.coinType !== undefined && state.coinType !== curState.coinType) {
        this._state = {
          ...this._state,
          isSyncing: isResyncingBlockchain(state.coinType),
          isResyncAvailable: isResyncAvailable(state.coinType),
        };
      }
    }
  }
}
</script>
<style lang="scss" scoped></style>
