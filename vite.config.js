import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  test: {
    globals: true,
    environment: 'jsdom',
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return;
          if (id.includes('@xterm')) return 'xterm';
          if (id.includes('react-markdown') || id.includes('remark-gfm')) return 'markdown';
          if (id.includes('framer-motion')) return 'motion';
          return 'vendor';
        }
      }
    }
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws/terminal': {
        target: 'http://localhost:8080',
        ws: true,
      },
    }
  }
})
