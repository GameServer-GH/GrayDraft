import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { useLocaleStore } from '@/stores/locale'
import { useAgentModelConfigStore } from '@/stores/agentModelConfig'
import '@/assets/styles/reset.css'
import '@/assets/styles/tokens.css'
import '@/assets/styles/glass.css'

const pinia = createPinia()
const app = createApp(App)
app.use(pinia)
useLocaleStore().hydrate()
useAgentModelConfigStore().hydrate()
app.mount('#app')
