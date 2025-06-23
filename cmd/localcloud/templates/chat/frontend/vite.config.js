// cmd/localcloud/templates/chat/frontend/vite.config.js
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
    plugins: [react()],
    server: {
        port: {{.FrontendPort}},
host: true,
    proxy: {
    '/api': {
        target: 'http://localhost:{{.APIPort}}',
            changeOrigin: true,
    }
}
},
build: {
    outDir: 'dist',
        sourcemap: true,
}
})