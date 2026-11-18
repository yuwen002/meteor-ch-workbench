import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 3000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/monaco-editor/esm/vs/languages/definitions')) return 'monaco-languages'
          if (id.includes('node_modules/monaco-editor')) return 'monaco-editor'
          if (id.includes('node_modules/vue') || id.includes('node_modules/pinia')) return 'vue-vendor'
        },
      },
    },
  },
  optimizeDeps: {
    include: ['monaco-editor/editor/editor.api.js'],
  },
})