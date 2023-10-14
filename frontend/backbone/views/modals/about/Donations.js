import qr from 'qr-encode';
import { ipc } from '../../../../src/utils/ipcRenderer.js';
import app from '../../../app';
import { openSimpleMessage } from '../../modals/SimpleMessage';
import {
  isSupportedWalletCur,
  getCurrencyByCode,
} from '../../../data/walletCurrencies';
import { launchWallet } from '../../../utils/modalManager';
import loadTemplate from '../../../utils/loadTemplate';
import baseVw from '../../baseVw';


let hiderTimer;

export default class extends baseVw {
  constructor(options = {}) {
    const opts = {
      ...options,
      initialState: {
        showCoin: 'BTC',
        ...options.initialState,
      },
    };

    super({
      className: 'aboutDonations',
      ...opts,
    });
    this.options = opts;

    const btcAddress = '3DRgGSpscQvZBmgV33zEkeYF7L71eH4HrD';
    const btcQRAddress = getCurrencyByCode('BTC').qrCodeText(btcAddress);
    const bchAddress = 'qp0xmudvwvswlcgh80pt98ysxph6r4wfggzeqh68hr';
    const bchQRAddress = getCurrencyByCode('BCH').qrCodeText(bchAddress);

    this.dCoins = {
      BTC: {
        obDonationAddress: btcAddress,
        qrCodeDataURI: qr(btcQRAddress, { type: 6, size: 6, level: 'Q' }),
        walletSupported: isSupportedWalletCur('BTC'),
      },
      BCH: {
        obDonationAddress: bchAddress,
        qrCodeDataURI: qr(bchQRAddress, { type: 6, size: 6, level: 'Q' }),
        walletSupported: isSupportedWalletCur('BCH'),
      },
    };
  }

  events() {
    return {
      'click .js-copyAddress': 'copyDonationAddress',
      'click .js-openInWallet': 'openInWalletClick',
      'click .js-btc': 'showBTC',
      'click .js-bch': 'showBCH',
    };
  }

  showBTC() {
    this.setState({ showCoin: 'BTC' });
  }

  showBCH() {
    this.setState({ showCoin: 'BCH' });
  }

  copyDonationAddress() {
    const addr = this.dCoins[this.getState().showCoin].obDonationAddress;
    ipc.send('controller.system.writeToClipboard', addr);
    const copyNotif = this.getCachedEl('.js-copyNotification');

    copyNotif.addClass('active');
    if (!!hiderTimer) {
      clearTimeout(hiderTimer);
    }
    hiderTimer = setTimeout(() => copyNotif.removeClass('active'), 3000);
  }

  openInWalletClick() {
    let wallet = launchWallet({
      initialActiveCoin: this.getState().showCoin,
      initialSendModeOn: true,
    });

    const sendView = wallet.getSendMoneyVw();

    if (sendView.saveInProgress) {
      openSimpleMessage(
        app.polyglot.t('about.donationsTab.unableToOpenInWallet.title'),
        app.polyglot.t('about.donationsTab.unableToOpenInWallet.body')
      );
    } else {
      const state = this.getState();
      wallet.activeCoin = state.showCoin;
      wallet.sendModeOn = true;
      sendView
        .setFormData({ address: this.dCoins[state.showCoin].obDonationAddress });
      wallet.open();
    }
  }

  render() {
    super.render();
    const showCoin = this.getState().showCoin;
    loadTemplate('modals/about/donations.html', (t) => {
      this.$el.html(t({
        showCoin,
        ...this.dCoins[showCoin],
      }));
    });

    return this;
  }
}
