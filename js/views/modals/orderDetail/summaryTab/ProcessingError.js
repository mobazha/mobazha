import loadTemplate from '../../../../utils/loadTemplate';
import BaseVw from '../../../baseVw';

export default class extends BaseVw {
  constructor(options = {}) {
    if (!options.orderID) {
      throw new Error('Please provide the order id.');
    }

    const opts = {
      initialState: {
        isBuyer: false,
        isModerated: false,
        isOrderCancelable: false,
        isDisputable: false,
        errors: [],
        ...options.initialState || {},
      },
    };

    super(opts);

    this.orderID = options.orderID;
  }

  className() {
    return 'rowLg clrTErr';
  }

  render() {
    loadTemplate('modals/orderDetail/summaryTab/processingError.html', (t) => {
      this.$el.html(t({
        ...this._state,
      }));
    });

    return this;
  }
}
