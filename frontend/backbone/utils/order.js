import $ from 'jquery';
import app from '../app';
import { Events } from 'backbone';
import OrderFulfillment from '../models/order/orderFulfillment/OrderFulfillment';
import { openSimpleMessage } from '../views/modals/SimpleMessage';
import OrderCompletion from '../models/order/orderCompletion/OrderCompletion';
import OrderDispute from '../models/order/OrderDispute';
import ResolveDispute from '../models/order/ResolveDispute';

const events = {
  ...Events,
};

const acceptPosts = {};
const rejectPosts = {};
const cancelPosts = {};
const fulfillPosts = {};
const refundPosts = {};
const completePosts = {};
const openDisputePosts = {};
const resolvePosts = {};
const acceptPayoutPosts = {};
const releaseEscrowPosts = {};

function confirmOrder(orderID, reject = false) {
  if (!orderID) {
    throw new Error('Please provide an orderID');
  }

  let post = acceptPosts[orderID];

  if (reject) {
    post = rejectPosts[orderID];
  }

  if (!post) {
    post = $.post({
      url: app.getServerUrl('ob/orderconfirmation'),
      data: JSON.stringify({
        orderID,
        reject,
      }),
      dataType: 'json',
      contentType: 'application/json',
    }).always(() => {
      if (reject) {
        delete rejectPosts[orderID];
      } else {
        delete acceptPosts[orderID];
      }
    }).done(() => {
      events.trigger(`${reject ? 'reject' : 'accept'}OrderComplete`, {
        id: orderID,
        xhr: post,
      });
    })
    .fail(xhr => {
      events.trigger(`${reject ? 'reject' : 'accept'}OrderFail`, {
        id: orderID,
        xhr: post,
      });

      const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
      openSimpleMessage(
        app.polyglot.t(`orderUtil.failed${reject ? 'Reject' : 'Accept'}Heading`),
        failReason
      );
    });

    if (reject) {
      rejectPosts[orderID] = post;
    } else {
      acceptPosts[orderID] = post;
    }

    events.trigger(`${reject ? 'rejecting' : 'accepting'}Order`, {
      id: orderID,
      xhr: post,
    });
  }

  return post;
}

export { events };

export function acceptingOrder(orderID) {
  return !!acceptPosts[orderID];
}

export function acceptOrder(orderID) {
  return confirmOrder(orderID);
}

export function rejectingOrder(orderID) {
  return !!rejectPosts[orderID];
}

export function rejectOrder(orderID) {
  return confirmOrder(orderID, true);
}

export function cancelingOrder(orderID) {
  return !!cancelPosts[orderID];
}

export function cancelOrder(orderID) {
  if (!orderID) {
    throw new Error('Please provide an orderID');
  }

  let post = cancelPosts[orderID];

  if (!post) {
    post = $.post({
      url: app.getServerUrl('ob/ordercancel'),
      data: JSON.stringify({
        orderID,
      }),
      dataType: 'json',
      contentType: 'application/json',
    }).always(() => {
      delete cancelPosts[orderID];
    }).done(() => {
      events.trigger('cancelOrderComplete', {
        id: orderID,
        xhr: post,
      });
    })
    .fail(xhr => {
      events.trigger('cancelOrderFail', {
        id: orderID,
        xhr: post,
      });

      const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
      openSimpleMessage(
        app.polyglot.t('orderUtil.failedCancelHeading'),
        failReason
      );
    });

    cancelPosts[orderID] = post;
    events.trigger('cancelingOrder', {
      id: orderID,
      xhr: post,
    });
  }

  return post;
}

export function fulfillingOrder(orderID) {
  return !!fulfillPosts[orderID];
}

export function fulfillOrder(contractType = 'PHYSICAL_GOOD', isLocalPickup = false, data = {}) {
  if (!data || !data.orderID) {
    throw new Error('An orderID must be provided with the data.');
  }

  const orderID = data.orderID;

  let post = fulfillPosts[orderID];

  if (!post) {
    const model = new OrderFulfillment(data, { contractType, isLocalPickup });
    post = model.save();

    if (!post) {
      Object.keys(model.validationError)
        .forEach(errorKey => {
          throw new Error(`${errorKey}: ${model.validationError[errorKey][0]}`);
        });
    } else {
      post.always(() => {
        delete fulfillPosts[orderID];
      }).done(() => {
        events.trigger('fulfillOrderComplete', {
          id: orderID,
          xhr: post,
        });
      })
      .fail(xhr => {
        events.trigger('fulfillOrderFail', {
          id: orderID,
          xhr: post,
        });

        const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
        openSimpleMessage(
          app.polyglot.t('orderUtil.failedFulfillHeading'),
          failReason
        );
      });

      fulfillPosts[orderID] = post;
      events.trigger('fulfillingOrder', {
        id: orderID,
        xhr: post,
      });
    }
  }

  return post;
}

export function refundingOrder(orderID) {
  return !!refundPosts[orderID];
}

export function refundOrder(orderID) {
  if (!orderID) {
    throw new Error('Please provide an orderID');
  }

  let post = refundPosts[orderID];

  if (!post) {
    post = $.post({
      url: app.getServerUrl('ob/orderrefund'),
      data: JSON.stringify({
        orderID,
      }),
      dataType: 'json',
      contentType: 'application/json',
    }).always(() => {
      delete refundPosts[orderID];
    }).done(() => {
      events.trigger('refundOrderComplete', {
        id: orderID,
        xhr: post,
      });
    })
    .fail(xhr => {
      events.trigger('refundOrderFail', {
        id: orderID,
        xhr: post,
      });

      const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
      openSimpleMessage(
        app.polyglot.t('orderUtil.failedRefundHeading'),
        failReason
      );
    });

    refundPosts[orderID] = post;
    events.trigger('refundingOrder', {
      id: orderID,
      xhr: post,
    });
  }

  return post;
}

/**
 * If the order with the given id is in the process of being completed, this method
 * will return an object containing the post xhr and the data that's being saved.
 */
export function completingOrder(orderID) {
  return !!completePosts[orderID];
}

export function completeOrder(orderID, data = {}) {
  if (!orderID) {
    throw new Error('Please provide an orderID');
  }

  if (!completePosts[orderID]) {
    const model = new OrderCompletion(data);
    const save = model.save();

    if (!save) {
      Object.keys(model.validationError)
        .forEach(errorKey => {
          throw new Error(`${errorKey}: ${model.validationError[errorKey][0]}`);
        });
    } else {
      save.always(() => {
        delete completePosts[orderID];
      }).done(() => {
        events.trigger('completeOrderComplete', {
          id: orderID,
          xhr: save,
        });
      })
      .fail(xhr => {
        events.trigger('completeOrderFail', {
          id: orderID,
          xhr: save,
        });

        const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
        openSimpleMessage(
          app.polyglot.t('orderUtil.failedCompleteHeading'),
          failReason
        );
      });

      completePosts[orderID] = {
        xhr: save,
        data: model.toJSON(),
      };
    }

    events.trigger('completingOrder', {
      id: orderID,
      xhr: save,
    });
  }

  return completePosts[orderID].xhr;
}

/**
 * If the order with the given id is in the process of a dispute being opened,
 * this method will return an object containing the post xhr and the data
 * that's being saved.
 */
export function openingDispute(orderID) {
  return !!openDisputePosts[orderID];
}

export function openDispute(orderID, data = {}) {
  if (!orderID) {
    throw new Error('Please provide an orderID');
  }

  if (!openDisputePosts[orderID]) {
    const model = new OrderDispute(data);
    const save = model.save();

    if (!save) {
      Object.keys(model.validationError)
        .forEach(errorKey => {
          throw new Error(`${errorKey}: ${model.validationError[errorKey][0]}`);
        });
    } else {
      save.always(() => {
        delete openDisputePosts[orderID];
      }).done(() => {
        events.trigger('openDisputeComplete', {
          id: orderID,
          xhr: save,
        });
      })
      .fail(xhr => {
        events.trigger('openDisputeFail', {
          id: orderID,
          xhr: save,
        });

        const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
        openSimpleMessage(
          app.polyglot.t('orderUtil.failedOpenDisputeHeading'),
          failReason
        );
      });

      openDisputePosts[orderID] = {
        xhr: save,
        data: model.toJSON(),
      };
    }

    events.trigger('openingDisputeOrder', {
      id: orderID,
      xhr: save,
    });
  }

  return openDisputePosts[orderID].xhr;
}

/**
 * If the order with the given id is in the process of its dispute being resolved,
 * this method will return an object containing the post xhr and the data that's
 * being saved.
 */
export function resolvingDispute(orderID) {
  return !!resolvePosts[orderID];
}

export function resolveDispute(model) {
  if (!(model instanceof ResolveDispute)) {
    throw new Error('model must be provided as an instance of a ResolveDispute model.');
  }

  if (!model.id) {
    throw new Error('The model must have an id set.');
  }

  const orderID = model.id;

  if (!resolvePosts[orderID]) {
    const save = model.save();

    if (!save) {
      Object.keys(model.validationError)
        .forEach(errorKey => {
          throw new Error(`${errorKey}: ${model.validationError[errorKey][0]}`);
        });
    } else {
      save.always(() => {
        delete resolvePosts[orderID];
      }).done(() => {
        events.trigger('resolveDisputeComplete', {
          id: orderID,
          xhr: save,
        });
      })
      .fail(xhr => {
        events.trigger('resolveDisputeFail', {
          id: orderID,
          xhr: save,
        });

        const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
        openSimpleMessage(
          app.polyglot.t('orderUtil.failedResolveHeading'),
          failReason
        );
      });

      resolvePosts[orderID] = {
        xhr: save,
        data: model.toJSON(),
      };
    }

    events.trigger('resolvingDispute', {
      id: orderID,
      xhr: save,
    });
  }

  return resolvePosts[orderID].xhr;
}

export function acceptingPayout(orderID) {
  return !!acceptPayoutPosts[orderID];
}

export function acceptPayout(orderID) {
  if (!orderID) {
    throw new Error('Please provide an orderID');
  }

  let post = acceptPayoutPosts[orderID];

  if (!post) {
    post = $.post({
      url: app.getServerUrl('ob/releasefunds'),
      data: JSON.stringify({
        orderID,
      }),
      dataType: 'json',
      contentType: 'application/json',
    }).always(() => {
      delete acceptPayoutPosts[orderID];
    }).done(() => {
      events.trigger('acceptPayoutComplete', {
        id: orderID,
        xhr: post,
      });
    })
    .fail(xhr => {
      events.trigger('acceptPayoutFail', {
        id: orderID,
        xhr: post,
      });

      const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
      openSimpleMessage(
        app.polyglot.t('orderUtil.failedAcceptPayoutHeading'),
        failReason
      );
    });

    acceptPayoutPosts[orderID] = post;
    events.trigger('acceptingPayout', {
      id: orderID,
      xhr: post,
    });
  }

  return post;
}

export function releasingEscrow(orderID) {
  return !!releaseEscrowPosts[orderID];
}

export function releaseEscrow(orderID) {
  if (!orderID) {
    throw new Error('Please provide an orderID');
  }

  let post = releaseEscrowPosts[orderID];

  if (!post) {
    post = $.post({
      url: app.getServerUrl('ob/releaseescrow'),
      data: JSON.stringify({
        orderID,
      }),
      dataType: 'json',
      contentType: 'application/json',
    }).always(() => {
      delete releaseEscrowPosts[orderID];
    }).done(() => {
      events.trigger('releaseEscrowComplete', {
        id: orderID,
        xhr: post,
      });
    })
    .fail(xhr => {
      events.trigger('releaseEscrowFail', {
        id: orderID,
        xhr: post,
      });

      const failReason = xhr.responseJSON && xhr.responseJSON.reason || '';
      openSimpleMessage(
        app.polyglot.t('orderUtil.failedReleaseEscrowHeading'),
        failReason
      );
    });

    releaseEscrowPosts[orderID] = post;
    events.trigger('releasingEscrow', {
      id: orderID,
      xhr: post,
    });
  }

  return post;
}
