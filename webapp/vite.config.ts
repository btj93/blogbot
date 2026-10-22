import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig(({ command }) => ({
  plugins: [react()],
  // The Go server mounts the production bundle under /tg/blogbot/; changing
  // this without changing the mount breaks every asset URL.
  base: command === 'build' ? '/tg/blogbot/' : '/',
  build: { outDir: 'build' },
  server: { port: 8080 },
}))
