import { myGet, myPost, myAjax } from './api';

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
    return myPost(app.getServerUrl(`ob/carts/${peerID}/add`), purchaseItem);
  },

  removeCartItem(peerID, purchaseItem) {
    return myPost(app.getServerUrl(`ob/carts/${peerID}/remove`), purchaseItem);
  },
};
