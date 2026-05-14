import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import App from './App.vue'
import './styles/main.scss'

// Router will be imported here after router/index.js is created
// import router from './router'

const app = createApp(App)

app.use(createPinia())
// app.use(router)  // Commented out until router is ready
app.use(ElementPlus, { locale: zhCn })

app.mount('#app')