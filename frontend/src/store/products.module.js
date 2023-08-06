const items = [
  {
    id: 1,
    qty: 1,
    name: 'Logitech’s wireless gaming headset 4 PC',
    image:
      'https://cdn.vox-cdn.com/thumbor/7feXoyqV0dA11Nb66oF-3OM6VyA=/0x0:2040x1360/920x0/filters:focal(0x0:2040x1360):format(webp):no_upscale()/cdn.vox-cdn.com/uploads/chorus_asset/file/7719503/logg533_s.jpg',
    price: 99709,
    discount: 1750
  },
  {
    id: 2,
    qty: 1,
    name: 'Sony’s wireless noise-canceling headphones',
    image:
      'https://cdn.vox-cdn.com/thumbor/yFX92Ay431yoCPAAJgDmzGmHp0g=/0x0:2040x1360/920x613/filters:focal(971x550:1297x876):format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/67430511/jbareham_180823_2895_0075.0.jpg',
    price: 63100,
    discount: 1200
  },
  {
    id: 3,
    qty: 1,
    name: 'Asus ROG Zephyrus gaming laptop',
    image:
      'https://cdn.vox-cdn.com/thumbor/I-bE3VI_GfkmcQf4aail372gdE0=/0x0:2040x1360/912x513/filters:focal(857x517:1183x843):format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/67389250/mchin_181204_4182_0003.0.0.jpg',
    price: 393000,
    discount: 6900
  },
  {
    id: 4,
    qty: 1,
    name: 'Samsung Galaxy smartwatch',
    image:
      'https://cdn.vox-cdn.com/thumbor/zondm8JgTzU0G1CJTQpDRKa1Oq0=/0x106:2040x1254/850x479/filters:format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/65416582/vpavic_191007_3716_4.0.jpg',
    price: 33999,
    discount: 2118
  },
  {
    id: 5,
    qty: 1,
    name: 'Fossil Gen 5 smartwatch',
    image:
      'https://cdn.vox-cdn.com/thumbor/aBaz6U89mA5wqHQ_Mkyhfo96mag=/0x106:2040x1254/850x479/filters:format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/65124824/akrales_190822_3612_0135.0.jpg',
    price: 21500,
    discount: 1049
  },
  {
    id: 6,
    qty: 1,
    name: 'Motorola Active Edge Phone',
    image:
      'https://cdn.vox-cdn.com/thumbor/ABJwazRV4K2PMZDU8XgWk7GRwnc=/0x90:1722x1059/850x479/filters:format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/67131779/cgartenberg_200728_4115_0003.0.0.jpg',
    price: 170990,
    discount: 1200
  },
  {
    id: 7,
    qty: 1,
    name: 'Fujifilm’s new Instax Square Camera (SQ1)',
    image:
      'https://cdn.vox-cdn.com/thumbor/ye6d7YpwvMd63NzqGqHlUIJP7M4=/0x0:2040x1360/912x513/filters:focal(857x517:1183x843):format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/67414803/bfarsace_200910_4188_0003.0.0.jpg',
    price: 530045,
    discount: 755
  },
  {
    id: 8,
    qty: 1,
    name: 'Blackmagic pocket cinema camera',
    image:
      'https://cdn.vox-cdn.com/thumbor/X_I_XLpz2ifGloE4-0-B-NadzHE=/0x0:2040x1361/1570x883/filters:focal(840x585:1166x911):format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/65434235/brose_190930_3714_0005.0.jpg',
    price: 468039,
    discount: 910.45
  },
  {
    id: 9,
    qty: 1,
    name: 'Trainer Socks Black',
    image:
      'https://cdn.vox-cdn.com/thumbor/YFnb9mlx_bEgPzQHjvvLAY0QRc0=/0x0:2040x1360/920x613/filters:focal(877x866:1203x1192):format(webp)/cdn.vox-cdn.com/uploads/chorus_image/image/66397697/akrales_181019_3014_0770.0.jpg',
    price: 143895,
    discount: 417
  }
]

// Products initial states
const state = {
  forSale: items,
  cartItems: []
}

// Product action
const actions = {
  addToCart ({ commit }, id) {
    commit('addToCart', id)
  },
  removeCartItem ({ commit }, index) {
    commit('removeCartItem', index)
  }
}

// Product Mutations
const mutations = {
  addToCart (state, id) {
    state.cartItems.push(id)
  },
  removeCartItem (state, index) {
    state.cartItems.splice(index, 1)
  }
}

// Exporting the product module
const products = {
  namespaced: true,
  state,
  actions,
  mutations
}
export default products
