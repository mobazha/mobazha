import $ from 'jquery';

export default {
  getShoppingCarts(params = {}) {
    return $.get({
      url: app.getServerUrl('ob/carts'),
      dataType: 'json',
      contentType: 'application/json',
    });
  },

  clearShoppingCarts(params = {}, callback) {
    $.ajax({
      url: window.app.getServerUrl('ob/carts'),
      type: 'DELETE',
      success: function (result) {
        if (callback) {
          callback();
        }
      },
    });
  },

  addToShoppingCart(peerID, purchaseItem) {
    return $.post({
      url: app.getServerUrl(`ob/carts/${peerID}/add`),
      data: JSON.stringify(purchaseItem),
      dataType: 'json',
      contentType: 'application/json',
    });
  },

  removeCartItem(peerID, purchaseItem) {
    return $.post({
      url: app.getServerUrl(`ob/carts/${peerID}/remove`),
      data: JSON.stringify(purchaseItem),
      dataType: 'json',
      contentType: 'application/json',
    });
  },
};
