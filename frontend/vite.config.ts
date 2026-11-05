import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import wails from '@wailsio/runtime/plugins/vite';
import path from 'path'
import { fileURLToPath } from 'url'
import { dirname } from 'path'

// if in ESM context
const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), wails("./bindings")],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    }
  }
})
