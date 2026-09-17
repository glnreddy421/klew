import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    include: ['src/**/*.test.js'],
    exclude: [
      '**/node_modules/**',
      // node:test runner files — run via `node --test`, not vitest
      'src/lib/clusterConnection.test.js',
      'src/lib/clusterContext.test.js',
      'src/lib/clusterVersion.test.js',
      'src/lib/investigationOverview.test.js',
      'src/lib/logPatterns.test.js',
      'src/lib/patternFilters.test.js',
      'src/lib/resourceCategoryIcons.test.js',
      'src/lib/scopeSearch.test.js',
      'src/lib/workloadOverview.test.js',
      'src/hooks/useNavigationHistory.test.js',
    ],
  },
})
