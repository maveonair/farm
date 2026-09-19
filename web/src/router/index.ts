import { createRouter, createWebHistory } from 'vue-router'

import ActivityView from '@/views/ActivityView.vue'
import InstanceView from '@/views/InstanceView.vue'
import InstancesView from '@/views/InstancesView.vue'
import OverviewView from '@/views/OverviewView.vue'
import PoolView from '@/views/PoolView.vue'
import PoolsView from '@/views/PoolsView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'overview', component: OverviewView },
    { path: '/pools', name: 'pools', component: PoolsView },
    { path: '/pools/:name', name: 'pool', component: PoolView },
    { path: '/instances', name: 'instances', component: InstancesView },
    { path: '/instances/:id', name: 'instance', component: InstanceView },
    { path: '/activity', name: 'activity', component: ActivityView },
  ],
})

export default router
