import path from 'node:path';
import { fileURLToPath } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const root = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': path.resolve(root, 'src') } },
  server: {
    allowedHosts: true,
    host: '0.0.0.0',
    port: 5279,
    // 默认指向本地 admin-api。临时连其他环境用 VITE_API_TARGET 覆盖
    // （如 VITE_API_TARGET=https://diy.cszqq.top pnpm dev）——不要把远端地址
    // 提交进来，否则所有人 pnpm dev 都会直接读写那个环境。
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://127.0.0.1:5280',
        changeOrigin: true,
      },
    },
  },
  test: { environment: 'jsdom', maxWorkers: 2, setupFiles: './src/test/setup.ts', testTimeout: 60000 },
});
