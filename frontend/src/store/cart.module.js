
function updateLocalStorage(cart) {
  localStorage.setItem('cart', JSON.stringify(cart))
}

const state = {
  cart: {},
  transactions: {
    type: 'purchases',
    model: {},
  }
}

const getters = {
  cartItems: (state) => {
    return state.cart
  },
}

const mutations = {
  updateCart (state, cart) {
    state.cart = cart
  },
  setTransactionsOrder(state, data) {
    state.transactions = data;
  }

  // updateCartFromLocalStorage(state) {
  //   const cart = localStorage.getItem('cart')
  //   if (cart) {
  //     state.cart = JSON.parse(cart)
  //   }
  // }
}

const actions = {}

const cart = {
  namespaced: true,
  state,
  getters,
  actions,
  mutations
}
export default cart
