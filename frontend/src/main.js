import { createApp } from 'vue'
import './style.css'
import App from './App.vue'

const app = createApp(App)

// 添加全局错误处理程序
app.config.errorHandler = (err, vm, info) => {
  console.error('全局错误捕获:', err)
  console.error('组件:', vm)
  console.error('错误信息:', info)
}

app.mount('#app')
