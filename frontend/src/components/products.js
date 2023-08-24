const products = [
  {
    id: 1,
    name: '大钱的商店',
    avatar: new URL('@/assets/img/avatar.png', import.meta.url).href,
    items: [
      {
        title: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        image: new URL('@/assets/img/exam.png', import.meta.url).href,
        options: [
          { name: 'size', value: 'large' },
          { name: 'color', value: 'red' },
        ],
        price: 30,
        quantity: 1,
        total: 10,
        status: 0,
      },
      {
        title: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        image: new URL('@/assets/img/exam.png', import.meta.url).href,
        options: [
          { name: 'size', value: 'large' },
          { name: 'color', value: 'red' },
        ],
        price: 40,
        quantity: 3,
        total: 10,
        status: 0,
      },
    ],
  },
  {
    id: 2,
    name: '张三的商店',
    avatar: new URL('@/assets/img/avatar.png', import.meta.url).href,
    items: [
      {
        title: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        image: new URL('@/assets/img/exam.png', import.meta.url).href,
        options: [
          { name: 'size', value: 'large' },
          { name: 'color', value: 'red' },
        ],
        price: 50,
        quantity: 1,
        total: 10,
        status: 1,
      },
      {
        title: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        image: new URL('@/assets/img/exam.png', import.meta.url).href,
        options: [
          { name: 'size', value: 'large' },
          { name: 'color', value: 'red' },
        ],
        price: 60,
        quantity: 1,
        total: 10,
        status: 1,
      },
    ],
  },
];

export { products };
