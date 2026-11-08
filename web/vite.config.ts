import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'
import { defineConfig } from 'vite'

export default defineConfig({
  // Absolute base: the pages are served from a root path, never `../`.
  base: '/',
  plugins: [react(), tailwindcss()],
  // The pages module embeds this directory (`all:dist`); the guard file there must survive a build.
  build: { outDir: '../internal/pages/embed/dist', emptyOutDir: false },
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, 'src') },
  },
})
