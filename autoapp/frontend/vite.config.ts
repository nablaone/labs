import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/query':  'http://localhost:8080',
      '/action': 'http://localhost:8080',
      '/schema': 'http://localhost:8080',
      '/ui':     'http://localhost:8080',
      '/ws':     { target: 'ws://localhost:8080', ws: true },
    },
  },
})
