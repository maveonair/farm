import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
	build: {
		outDir: '../internal/server/dist/web',
		emptyOutDir: true,
	},
	server: {
		proxy: {
			'/api': 'http://127.0.0.1:8080',
			'/healthz': 'http://127.0.0.1:8080',
		},
	},
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
