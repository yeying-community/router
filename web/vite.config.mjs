import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5181,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:3011',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined;
          }
          if (
            id.includes('/react/') ||
            id.includes('/react-dom/') ||
            id.includes('/scheduler/')
          ) {
            return 'react';
          }
          if (
            id.includes('/react-router/') ||
            id.includes('/react-router-dom/') ||
            id.includes('/history/')
          ) {
            return 'router';
          }
          if (
            id.includes('/antd/') ||
            id.includes('/rc-') ||
            id.includes('@ant-design/') ||
            id.includes('/@babel/runtime/')
          ) {
            return 'antd';
          }
          if (
            id.includes('recharts') ||
            id.includes('recharts-scale') ||
            id.includes('victory-vendor') ||
            id.includes('/d3-') ||
            id.includes('/internmap/') ||
            id.includes('/react-smooth/')
          ) {
            return 'charts';
          }
          if (
            id.includes('i18next') ||
            id.includes('react-i18next')
          ) {
            return 'i18n';
          }
          if (
            id.includes('@yeying-community/web3-bs') ||
            id.includes('react-turnstile')
          ) {
            return 'web3';
          }
          return undefined;
        },
      },
    },
  },
});
