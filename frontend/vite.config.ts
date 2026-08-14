import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: process.env.NM_SOURCEMAP === '1',
    rollupOptions: {
      output: {
        manualChunks: {
          map: ['maplibre-gl', '@deck.gl/mapbox', '@deck.gl/layers', '@deck.gl/core'],
          charts: ['uplot'],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/upload-logs': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/upload-geo': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/upload-reputation': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/health': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/live': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/ready': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    setupFiles: ['./src/test/setup.ts'],
  },
});