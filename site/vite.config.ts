import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Relative base so the built site works when deployed under a subpath
// (GitHub Pages project sites) as well as at a domain root (Cloudflare Pages).
export default defineConfig({
  plugins: [react()],
  base: './',
})
