/**
 * 基础路由
 * @type { *[] }
 */

const constantRouterMap = [
  {
    path: '/',
    name: 'Example',
    redirect: { name: 'ExampleHelloIndex' },
    children: [
      {
        path: '/example',
        name: 'ExampleHelloIndex',
        component: () => import('@/views/example/hello/Index.vue')
      },
      {
        path: 'transactions',
        name: 'Transactions',
        component: () => import('@/views/transactions/Transactions.vue')
      },
      {
        path: 'transactions/:tab',
        name: 'Transactions',
        component: () => import('@/views/transactions/Transactions.vue')
      },
      {
        path: 'shopping-cart',
        name: 'ShoppingCart',
        component: () => import('@/views/ShoppingCart.vue')
      },
      {
        path: 'purchase',
        name: 'Purchase',
        component: () => import('@/views/modals/purchase/Purchase.vue')
      },
    ]
  },
]

export default constantRouterMap