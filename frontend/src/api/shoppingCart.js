import $ from 'jquery';
import { myGet, myAjax } from './api';

export default {
  getShoppingCarts() {
    return myGet(app.getServerUrl('ob/carts'));
  },

  clearShoppingCarts(params = {}, callback) {
    myAjax({
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
