<template>
  <div class="test-component">
    <h2>测试组件</h2>
    <p>这是一个测试组件，用于测试组件切换功能</p>
    <p>组件名称: {{ componentName }}</p>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'

const props = defineProps({
  componentName: {
    type: String,
    default: 'test'
  }
})

onMounted(() => {
  console.log(`${props.componentName} 组件已挂载`)
})
</script>

<style scoped>
.test-component {
  padding: 20px;
  background-color: var(--bg-secondary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
}
</style>