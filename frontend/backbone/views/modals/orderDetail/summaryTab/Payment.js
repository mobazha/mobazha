import $ from 'jquery';
import moment from 'moment';
import bigNumber from 'bignumber.js';
import app from '../../../../app';
import { abbrNum } from '../../../../utils';
import loadTemplate from '../../../../utils/loadTemplate';
import { integerToDecimal } from '../../../../utils/currency';
import BaseVw from '../../../baseVw';

export default class extends BaseVw {
  constructor(options = {}) {
    super({
      ...options,
      initialState: {
        paymentNumber: 1,
        amountShort: bigNumber(0),
        balanceRemaining: bigNumber(0),
        payee: '',
        userCurrency: app.settings.get('localCurrency') || 'BTC',
        showAcceptRejectButtons: false,
        showCancelButton: false,
        acceptInProgress: false,
        rejectInProgress: false,
        cancelInProgress: false,
        rejectConfirmOn: false,
        blockChainTxUrl: '',
        paymentCoin: '',
        paymentCoinDivis: 8,
        ...options.initialState || {},
      },
    });

    if (!this.model) {
      throw new Error('Please provide a model.');
    }

    this.boundOnDocClick = this.onDocumentClick.bind(this);
    $(document).on('click', this.boundOnDocClick);
  }

  className() {
    return 'payment rowLg';
  }

  events() {
    return {
      'click .js-cancelOrder': 'onClickCancelOrder',
      'click .js-acceptOrder': 'onClickAcceptOrder',
      'click .js-rejectOrder': 'onClickRejectOrder',
      'click .js-rejectConfirmed': 'onClickRejectConfirmed',
      'click .js-rejectConfirm': 'onClickRejectConfirmBox',
      'click .js-rejectConfirmCancel': 'onClickRejectConfirmCancel',
    };
  }

  onClickCancelOrder() {
    this.trigger('cancelClick', { view: this });
  }

  onClickAcceptOrder() {
    this.trigger('acceptClick', { view: this });
  }

  onClickRejectConfirmed() {
    this.trigger('confirmedRejectClick', { view: this });
    this.setState({ rejectConfirmOn: false });
  }

  onClickRejectOrder() {
    this.setState({ rejectConfirmOn: true });
    return false;
  }

  onClickRejectConfirmBox() {
    // ensure event doesn't bubble so onDocumentClick doesn't
    // close the confirmBox.
    return false;
  }

  onClickRejectConfirmCancel() {
    this.setState({ rejectConfirmOn: false });
  }

  onDocumentClick() {
    this.setState({ rejectConfirmOn: false });
  }

  remove() {
    $(document).off('click', this.boundOnDocClick);
    super.remove();
  }

  render() {
    const coinInfo = app.walletBalances.get(this._state.paymentCoin);
    let confirmations = 0;
    if (coinInfo && coinInfo.get('height') !== 0 && (+this.model.get('height'))) {
      confirmations = coinInfo.get('height') - this.model.get('height');
    }
    loadTemplate('modals/orderDetail/summaryTab/payment.html', (t) => {
      this.$el.html(t({
        ...this._state,
        ...this.model.toJSON(),
        value: integerToDecimal(this.model.get('value'), this._state.paymentCoinDivis),
        confirmations,
        abbrNum,
        moment,
      }));
    });

    return this;
  }
}
