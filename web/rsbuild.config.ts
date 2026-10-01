import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { defineConfig, loadEnv } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { pluginTailwindcss } from '@rsbuild/plugin-tailwindcss'
import { tanstackRouter } from '@tanstack/router-plugin/rspack'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] })
  const serverUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    'http://localhost:3000'

  const isProd = envMode === 'production'

  // Deployment prefix, read from the same BASE_PATH the Go server reads so the
  // build and the runtime agree on one contract. Baking it in emits absolute
  // /cuberouter/static/... references and gives the SPA a prefix to fall back on
  // when nothing injects window.__BASE_PATH__ (a static host, or a server that
  // predates the injection). Left empty -- the default -- the build stays
  // prefix-agnostic and is served correctly under any prefix by InjectBasePath.
  const trimmedBasePath = (process.env.BASE_PATH ?? '')
    .trim()
    .replaceAll(/^\/+|\/+$/g, '')
  // Only production bakes the prefix; `bun run dev` keeps serving from the root.
  const compiledBasePath =
    isProd && trimmedBasePath ? `/${trimmedBasePath}` : ''
  const builtBase = compiledBasePath === '' ? '/' : `${compiledBasePath}/`

  const devProxy = Object.fromEntries(
    (['/api', '/v1', '/mj', '/pg', '/swagger'] as const).map((key) => [
      key,
      { target: serverUrl, changeOrigin: true },
    ])
  ) as Record<string, { target: string; changeOrigin: boolean }>

  return {
    plugins: [pluginReact(), pluginTailwindcss({ optimize: false })],
    // Rsbuild 2: replaces deprecated `performance.chunkSplit` (RSPack 2 aligned)
    splitChunks: {
      preset: 'default',
      cacheGroups: {
        'vendor-react': {
          test: /node_modules[\\/](react|react-dom)[\\/]/,
          name: 'vendor-react',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        'vendor-ui-primitives': {
          test: /node_modules[\\/](@base-ui|@radix-ui)[\\/]/,
          name: 'vendor-ui-primitives',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        'vendor-tanstack': {
          test: /node_modules[\\/]@tanstack[\\/]/,
          name: 'vendor-tanstack',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
      },
    },
    source: {
      entry: {
        index: './src/main.tsx',
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    html: {
      template: './index.html',
      // The one hand-written absolute reference in the template: the favicon is
      // copied from public/ verbatim, so it is not rewritten by assetPrefix.
      templateParameters: { appBasePath: compiledBasePath },
    },
    server: {
      host: '0.0.0.0',
      strictPort: false,
      // server.base is Vite's `base`: it prefixes the emitted asset URLs and
      // surfaces to the app as import.meta.env.BASE_URL.
      base: builtBase,
      proxy: devProxy,
    },
    output: {
      // Production optimizations
      minify: isProd,
      target: 'web',
      // "auto" makes the rspack runtime derive __webpack_public_path__ from the URL of the
      // script currently executing, so lazily-loaded chunks, worker URLs and CSS url()
      // references resolve correctly whether the app is served from the site root or from a
      // URL path prefix (BASE_PATH). It also makes the generated index.html reference assets
      // relatively (static/js/... instead of /static/js/...); the Go server normalises those
      // back to a base-prefixed absolute form at startup (InjectBasePath in main.go), which
      // also protects deep-link reloads such as /dashboard/settings where a relative
      // reference would otherwise resolve against the current directory.
      //
      // A build run with BASE_PATH set instead pins the prefix outright: the emitted
      // references become absolute (/cuberouter/static/...) so the output is correct even
      // on a host that never touches the HTML.
      assetPrefix: compiledBasePath === '' ? 'auto' : builtBase,
      distPath: {
        root: 'dist',
      },
      // Rely on Rsbuild default legalComments ("linked" → per-chunk *.LICENSE.txt) in all modes.
      // Do not set "none" in production: that strips minifier-preserved third-party notices and
      // extracted license files, which some distributions require for open-source compliance.
    },
    performance: {
      // Remove console in production
      removeConsole: isProd ? ['log'] : false,
      buildCache: false,
    },
    tools: {
      rspack: {
        plugins: [
          tanstackRouter({
            target: 'react',
            // Dev: avoid per-route async chunks (reduces white flash on navigation + faster HMR feedback).
            // Prod: keep route-based code splitting.
            autoCodeSplitting: isProd,
          }),
        ],
      },
    },
  }
})
