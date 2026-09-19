import { createApp } from 'vue'
import { VueQueryPlugin } from '@tanstack/vue-query'
import App from './App.vue'
import router from './router'
import './styles.css'

import { newQueryClient } from '@/queries/client'

const app = createApp(App)

app.use(router)
app.use(VueQueryPlugin, { queryClient: newQueryClient() })

app.mount('#app')
