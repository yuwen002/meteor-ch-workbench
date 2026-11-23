import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config'

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environmentMatchGlobs: [
        ['src/components/**/*.test.ts', 'jsdom'],
        ['src/utils/**/*.test.ts', 'node'],
      ],
      environment: 'node',
      include: ['src/**/*.test.ts'],
    },
  }),
)