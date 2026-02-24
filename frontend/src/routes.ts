import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    path: '/sharing',
    name: 'Sharing',
    component: () => import('shell/app-layout'),
    redirect: '/sharing/links',
    meta: {
      order: 2040,
      icon: 'lucide:share-2',
      title: 'sharing.menu.sharing',
      keepAlive: true,
      authority: ['platform:admin', 'tenant:manager'],
    },
    children: [
      {
        path: 'links',
        name: 'SharingLinks',
        meta: {
          icon: 'lucide:link',
          title: 'sharing.menu.links',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/links/index.vue'),
      },
      {
        path: 'templates',
        name: 'SharingTemplates',
        meta: {
          icon: 'lucide:mail',
          title: 'sharing.menu.templates',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/templates/index.vue'),
      },
    ],
  },
];

export default routes;
