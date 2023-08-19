const products = [
  {
    id: 1,
    name: '大钱的商店',
    avatar: new URL('@/assets/img/avatar.png', import.meta.url).href,
    children: [
      {
        name: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        url: new URL('@/assets/img/exam.png', import.meta.url).href,
        sku: [
          { label: 'size', value: 'large' },
          { label: 'color', value: 'red' },
        ],
        price: 30,
        num: 1,
        total: 10,
        status: 0,
      },
      {
        name: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        url: new URL('@/assets/img/exam.png', import.meta.url).href,
        sku: [
          { label: 'size', value: 'large' },
          { label: 'color', value: 'red' },
        ],
        price: 40,
        num: 3,
        total: 10,
        status: 0,
      },
    ],
  },
  {
    id: 2,
    name: '张三的商店',
    avatar: new URL('@/assets/img/avatar.png', import.meta.url).href,
    children: [
      {
        name: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        url: new URL('@/assets/img/exam.png', import.meta.url).href,
        sku: [
          { label: 'size', value: 'large' },
          { label: 'color', value: 'red' },
        ],
        price: 50,
        num: 1,
        total: 10,
        status: 1,
      },
      {
        name: 'Jiuquan 10000mAh mobile power with cable YY-1S',
        url: new URL('@/assets/img/exam.png', import.meta.url).href,
        sku: [
          { label: 'size', value: 'large' },
          { label: 'color', value: 'red' },
        ],
        price: 60,
        num: 1,
        total: 10,
        status: 1,
      },
    ],
  },
];

export { products };
