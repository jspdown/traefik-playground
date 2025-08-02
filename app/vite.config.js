import { resolve } from 'path'
import { defineConfig } from 'vite'

export default defineConfig({
    build: {
        lib: {
            entry: resolve(__dirname, 'js/index.js'),
            name: 'traefik-playground',
            fileName: 'index',
        }
    },
    plugins: []
})
