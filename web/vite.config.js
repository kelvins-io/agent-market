import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 7100,
    proxy: {
      '/api': 'http://127.0.0.1:13100',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
