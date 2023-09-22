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
      // {
      //   path: '/^(?:ob:\/\/)(12D3Koo[a-zA-Z0-9]+)[\/]?([^\/]*)[\/]?([^\/]*)[\/]?([^\/]*)\/?$/',
      //   name: 'UserPage',
      //   component: () => import('@/views/userPage/UserPage.vue')
      // },
      // {
      //   path: '/:guid(12D3Koo[a-zA-Z0-9]+)',
      //   name: 'UserPage',
      //   component: () => import('@/views/userPage/UserPage.vue')
      // },
    ]
  },
]

export default constantRouterMap