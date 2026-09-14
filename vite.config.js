import {defineConfig} from 'vite';

export default defineConfig({
  build: {
    outDir: 'web',
    emptyOutDir: false,
    rollupOptions: {
      input: 'web/src/main.js',
      output: {
        entryFileNames: 'dashboard.js',
        assetFileNames: '[name][extname]',
        chunkFileNames: '[name].js',
        format: 'es'
      }
    },
    minify: false,
    sourcemap: false
  }
});
