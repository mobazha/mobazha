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
        path: '/:guid(12D3Koo[a-zA-Z0-9]+)/:state?/:slug?',
        name: 'UserPage',
        component: () => import('@/views/userPage/UserPage.vue')
      },
      {
        path: 'search/:tab?',
        name: 'Search',
        component: () => import('@/views/search/Search.vue')
      },
      {
        path: 'connected-peers',
        name: 'connectedPeers',
        component: () => import('@/views/ConnectedPeersPage.vue')
      },
    ]
  },
]

export default constantRouterMap