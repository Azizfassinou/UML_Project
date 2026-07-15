import { createRouter, createWebHistory } from 'vue-router';
import { authState } from './auth.js';

import Login from './views/Login.vue';
import Register from './views/Register.vue';
import Dashboard from './views/Dashboard.vue';

const routes = [
  {
    path: '/',
    redirect: '/dashboard',
  },
  {
    path: '/login',
    name: 'Login',
    component: Login,
    meta: { requiresGuest: true },
  },
  {
    path: '/register',
    name: 'Register',
    component: Register,
    meta: { requiresGuest: true },
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: Dashboard,
    meta: { requiresAuth: true },
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/dashboard',
  }
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

// Guard de navigation pour gérer l'authentification
router.beforeEach((to, from, next) => {
  const isAuthenticated = authState.isAuthenticated;

  if (to.matched.some(record => record.meta.requiresAuth)) {
    if (!isAuthenticated) {
      next({ name: 'Login' });
    } else {
      next();
    }
  } else if (to.matched.some(record => record.meta.requiresGuest)) {
    if (isAuthenticated) {
      next({ name: 'Dashboard' });
    } else {
      next();
    }
  } else {
    next();
  }
});

export default router;
