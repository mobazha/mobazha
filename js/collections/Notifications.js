/* eslint-disable class-methods-use-this */
import _ from 'underscore';
import moment from 'moment';
import { Collection } from 'backbone';
import { capitalize } from '../utils/string';
import app from '../app';

export default class extends Collection {
  url() {
    return app.getServerUrl('ob/notifications');
  }

  parse(response) {
    return response.notifications.map((notif) => {
      const innerNotif = notif.notification;

      return {
        id: innerNotif.notificationID,
        notification: _.omit(innerNotif, 'notificationID'),
        ...notif,
      };
    });
  }

  comparator(message) {
    return message.get('timestamp');
  }
}

/**
 * Based on a notification's data, this function will determine what text
 * the notification should be displayed with and what route it should link to.
 * Based on option.native, it will tailor the text for it to be used on a native
 * JS notification or our internal app one (the former can't contain html).
 *
 * @param {object} attrs - The notification data, If you have a Notification model,
 *   then this is the embedded notification object (i.e. this.model.toJSON().notification).
 * @param {object} [options={}]
 * @return {object} An object containting text and route properties.
 */
export function getNotifDisplayData(attrs, options = {}) {
  if (typeof attrs !== 'object') {
    throw new Error('Please provide an object with notification data.');
  }

  const opts = {
    native: false,
    ...options,
  };

  let text = '';
  let route = '';

  const getName = (handle, guid) => (handle && `@${handle}`) || `${guid.slice(0, 8)}…`;

  if (attrs.type === 'newOrder') {
    const buyerName = opts.native
      ? getName(attrs.buyerHandle, attrs.buyerID)
      : `<a class="clrTEm" href="#${attrs.buyerID}">${getName(attrs.buyerHandle, attrs.buyerID)}</a>`;
    const listingTitle = opts.native
      ? attrs.title
      : `<a class="clrTEm" href="#${app.profile.id}/store/${attrs.slug}">${attrs.title}</a>`;

    route = `#transactions/sales?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.order', {
      buyerName,
      listingTitle,
    });
  } else if (attrs.type === 'orderFunded') {
    const listingTitle = opts.native
      ? attrs.title
      : `<a class="clrTEm" href="#${app.profile.id}/store/${attrs.slug}">${attrs.title}</a>`;
    route = `#transactions/sales?orderID=${attrs.orderID}`;
    text = `${listingTitle} has been funded.`;
  } else if (attrs.type === 'orderPaymentReceived') {
    route = `#transactions/purchases?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.payment');
  } else if (attrs.type === 'orderConfirmation') {
    const vendorName = opts.native
      ? getName(attrs.vendorHandle, attrs.vendorID)
      : `<a class="clrTEm" href="#${attrs.vendorID}">`
        + `${getName(attrs.vendorHandle, attrs.vendorID)}</a>`;
    route = `#transactions/purchases?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.orderConfirmation', {
      vendorName,
    });
  } else if (attrs.type === 'orderDeclined') {
    const vendorName = opts.native
      ? getName(attrs.vendorHandle, attrs.vendorID)
      : `<a class="clrTEm" href="#${attrs.vendorID}">`
        + `${getName(attrs.vendorHandle, attrs.vendorID)}</a>`;
    route = `#transactions/purchases?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.declined', {
      vendorName,
    });
  } else if (attrs.type === 'orderCancel') {
    const buyerName = opts.native
      ? getName(attrs.buyerHandle, attrs.buyerID)
      : `<a class="clrTEm" href="#${attrs.buyerID}">`
        + `${getName(attrs.buyerHandle, attrs.buyerID)}</a>`;
    route = `#transactions/sales?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.canceled', {
      buyerName,
    });
  } else if (attrs.type === 'refund') {
    const vendorName = opts.native
      ? getName(attrs.vendorHandle, attrs.vendorID)
      : `<a class="clrTEm" href="#${attrs.vendorID}">`
        + `${getName(attrs.vendorHandle, attrs.vendorID)}</a>`;
    route = `#transactions/purchases?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.refunded', {
      vendorName,
    });
  } else if (attrs.type === 'orderFulfillment') {
    const vendorName = opts.native
      ? getName(attrs.vendorHandle, attrs.vendorID)
      : `<a class="clrTEm" href="#${attrs.vendorID}">`
        + `${getName(attrs.vendorHandle, attrs.vendorID)}</a>`;
    route = `#transactions/purchases?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.fulfillment', {
      vendorName,
    });
  } else if (attrs.type === 'orderCompletion') {
    const buyerName = opts.native
      ? getName(attrs.buyerHandle, attrs.buyerID)
      : `<a class="clrTEm" href="#${attrs.buyerID}">${getName(attrs.buyerHandle, attrs.buyerID)}</a>`;
    route = `#transactions/sales?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.orderComplete', {
      buyerName,
    });
  } else if (attrs.type === 'processingError') {
    const vendorName = opts.native
      ? getName(attrs.vendorHandle, attrs.vendorID)
      : `<a class="clrTEm" href="#${attrs.vendorID}">`
        + `${getName(attrs.vendorHandle, attrs.vendorID)}</a>`;
    route = `#transactions/purchases?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.processingError', {
      vendorName,
    });
  } else if (attrs.type === 'caseOpen') {
    if (attrs.disputeeID === app.profile.id) {
      // notif received by disputee
      const disputerName = opts.native
        ? getName(attrs.disputerHandle, attrs.disputerID)
        : `<a class="clrTEm" href="#${attrs.disputerID}">`
          + `${getName(attrs.disputerHandle, attrs.disputerID)}</a>`;
      route = `#transactions/${attrs.buyer === attrs.disputerID ? 'purchases' : 'sales'}`
        + `?orderID=${attrs.caseID}`;
      text = app.polyglot.t('notifications.text.disputeOpen', {
        disputerName,
      });
    } else {
      // you are the mod receiving this notification
      const disputerName = opts.native
        ? getName(attrs.disputerHandle, attrs.disputerID)
        : `<a class="clrTEm" href="#${attrs.disputerID}">`
          + `${getName(attrs.disputerHandle, attrs.disputerID)}</a>`;
      const disputeeName = opts.native
        ? getName(attrs.disputeeHandle, attrs.disputeeID)
        : `<a class="clrTEm" href="#${attrs.disputeeID}">`
          + `${getName(attrs.disputeeHandle, attrs.disputeeID)}</a>`;

      route = `#transactions/cases?caseID=${attrs.caseID}`;
      text = app.polyglot.t('notifications.text.disputeOpenMod', {
        disputerName,
        disputeeName,
      });
    }
  } else if (attrs.type === 'caseUpdate') {
    const disputerName = opts.native
      ? getName(attrs.disputerHandle, attrs.disputerID)
      : `<a class="clrTEm" href="#${attrs.disputerID}">`
        + `${getName(attrs.disputerHandle, attrs.disputerID)}</a>`;
    const disputeeName = opts.native
      ? getName(attrs.disputeeHandle, attrs.disputeeID)
      : `<a class="clrTEm" href="#${attrs.disputeeID}">`
        + `${getName(attrs.disputeeHandle, attrs.disputeeID)}</a>`;
    route = `#transactions/cases?orderID=${attrs.caseID}`;
    text = app.polyglot.t('notifications.text.disputeUpdate', {
      disputerName,
      disputeeName,
    });
  } else if (attrs.type === 'disputeClose') {
    const otherPartyName = opts.native
      ? getName(attrs.otherPartyHandle, attrs.otherPartyID)
      : `<a class="clrTEm" href="#${attrs.otherPartyID}">`
        + `${getName(attrs.otherPartyHandle, attrs.otherPartyID)}</a>`;
    route = `#transactions/${attrs.buyer === attrs.otherPartyID ? 'purchases' : 'sales'}`
      + `?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.disputeClose', {
      otherPartyName,
    });
  } else if (attrs.type === 'disputeAccepted') {
    const otherPartyName = opts.native
      ? getName(attrs.otherPartyHandle, attrs.otherPartyID)
      : `<a class="clrTEm" href="#${attrs.buyerID}">`
        + `${getName(attrs.otherPartyHandle, attrs.otherPartyID)}</a>`;
    route = `#transactions/${attrs.buyer === attrs.otherPartyID ? 'purchases' : 'sales'}`
      + `?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.disputeAccepted', {
      otherPartyName,
    });
  } else if (attrs.type === 'follow' || attrs.type === 'unfollow' || attrs.type === 'moderatorAdd'
    || attrs.type === 'moderatorRemove') {
    const name = opts.native
      ? getName(attrs.handle, attrs.peerID)
      : `<a class="clrTEm" href="#${attrs.peerID}">${getName(attrs.handle, attrs.peerID)}</a>`;
    route = `#${attrs.peerID}`;
    text = app.polyglot.t(`notifications.text.${attrs.type}`, {
      name,
    });
  } else if ([
    'vendorDisputeTimeout',
    'buyerDisputeTimeout',
    'buyerDisputeExpiry',
    'moderatorDisputeExpiry',
  ].includes(attrs.type)) {
    const prevMomentDaysThreshold = moment.relativeTimeThreshold('d');
    let orderIDKey = 'orderID';

    if (attrs.type === 'vendorDisputeTimeout') {
      orderIDKey = 'purchaseOrderID';
    } else if (attrs.type === 'moderatorDisputeExpiry') {
      orderIDKey = 'disputeCaseID';
    }

    const orderIDShort = `#${attrs[orderIDKey].slice(0, 4)}…`;
    let transactionTab = 'sales';
    let orderApiFilter = 'orderID';

    if ([
      'buyerDisputeTimeout',
      'buyerDisputeExpiry',
    ].includes(attrs.type)) {
      transactionTab = 'purchases';
    } else if (attrs.type === 'moderatorDisputeExpiry') {
      transactionTab = 'cases';
      orderApiFilter = 'caseID';
    }

    route = `#transactions/${transactionTab}?${orderApiFilter}=${attrs[orderIDKey]}`;

    if (attrs.expiresIn > 0) {
      // temporarily upping the moment threshold of number of days before month is used,
      // so e,g. 45 is represented as '45 days' instead of '1 month'.
      moment.relativeTimeThreshold('d', 364);

      const timeRemaining = moment(Date.now())
        .from(Date.now() - (attrs.expiresIn * 1000), true);

      text = app.polyglot.t(`notifications.text.${attrs.type}`, {
        orderLink: opts.native
          ? orderIDShort
          : `<a href="${route}" class="clrTEm">${orderIDShort}</a>`,
        timeRemaining,
      });

      text = text.startsWith(timeRemaining) ? capitalize(text) : text;

      // restore the days timeout threshold
      moment.relativeTimeThreshold('d', prevMomentDaysThreshold);
    } else {
      text = app.polyglot.t(`notifications.text.${attrs.type}Expired`, {
        orderLink: opts.native
          ? orderIDShort
          : `<a href="${route}" class="clrTEm">${orderIDShort}</a>`,
      });
    }
  } else if (attrs.type === 'vendorFinalizedPayment') {
    const orderIDShort = `#${attrs.orderID.slice(0, 4)}…`;
    route = `#transactions/purchases?orderID=${attrs.orderID}`;
    text = app.polyglot.t('notifications.text.vendorFinalizedPayment', {
      orderLink: opts.native
        ? orderIDShort
        : `<a href="${route}" class="clrTEm">${orderIDShort}</a>`,
    });
  }

  return {
    text,
    route,
  };
}
