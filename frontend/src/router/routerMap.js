/**
 * 基础路由
 * @type { *[] }
 */

import Profile from '../../backbone/models/profile/Profile';
import app from '../../backbone/app';

const constantRouterMap = [
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/userPage/UserPage.vue'),
    props: {bb: function() {
      return {
        model: app.profile,
      };
    }},
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
      // https://router.vuejs-korea.org/guide/essentials/route-matching-syntax.html#optional-parameters
      {
        path: '/:guid(12D3Koo[a-zA-Z0-9]+)/:state?/:slug?',
        name: 'UserPage',
        component: () => import('@/views/userPage/UserPage.vue'),
        meta: {
          watchParam: 'guid',
        },
        props: route => ({bb: function() {
          let { guid } = route.params;

          let model;
          if (guid === app.profile.id) {
            model = app.profile;
          } else {
            model = new Profile({ peerID: guid });
          }
          return {
            model,
          };
        }}),
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
      // {
      //   path: 'connected-peers',
      //   name: 'pageNotFound',
      //   component: () => import('@/views/pageNotFound.vue')
      // }
      // will match everything and put it under `$route.params.pathMatch`
      {
        path: '/:pathMatch(.*)*',
        name: 'PageNotFound',
        component: () => import('@/views/error-pages/PageNotFound.vue')
      },
    ]
  },
]

export default constantRouterMap