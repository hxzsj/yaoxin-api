/**
 * SRI (Subresource Integrity) Plugin for Rsbuild 2.x
 *
 * Automatically adds `integrity` and `crossorigin` attributes to all
 * <script> and <link rel="stylesheet"> tags in production builds.
 * Uses SHA-384 as the hash algorithm (SRI recommended minimum).
 *
 * Only activates during `rsbuild build` (apply: 'build'), not in dev server.
 */
import { createHash } from 'node:crypto'
import { basename } from 'node:path'
import type { RsbuildPlugin } from '@rsbuild/core'

export function pluginSRI(): RsbuildPlugin {
  return {
    name: 'sri-integrity',
    apply: 'build',
    setup(api) {
      api.modifyHTML((html, context) => {
        const { compilation } = context

        // Build filename -> sha384-base64 integrity map from compilation assets (in-memory)
        const integrityMap = new Map<string, string>()
        for (const [filename, source] of Object.entries(compilation.assets)) {
          if (filename.endsWith('.js') || filename.endsWith('.css')) {
            const content =
              typeof source === 'string' ? source : (source.source() as string)
            const hash = createHash('sha384').update(content).digest('base64')
            integrityMap.set(basename(filename), `sha384-${hash}`)
          }
        }

        // No assets found — return unchanged
        if (integrityMap.size === 0) return html

        // Inject integrity + crossorigin into <script src="..."> tags
        let result = html.replace(
          /(<script\s[^>]*)(src=["']([^"']+)["'])/g,
          (_match, prefix, _srcAttr, url) => {
            const fileName = basename(url)
            const integrity = integrityMap.get(fileName)
            if (!integrity) return _match
            return `${prefix}src="${url}" integrity="${integrity}" crossorigin="anonymous"`
          },
        )

        // Inject into <link rel="stylesheet" ...> tags (handle any attr order)
        result = result.replace(
          /(<link\s(?:[^>]*?\s)?)(href=["']([^"']+)["'])([^>]*)(rel=["']stylesheet["'])/g,
          (_match, prefix, hrefAttr, url, midAttrs, _relAttr) => {
            const fileName = basename(url)
            const integrity = integrityMap.get(fileName)
            if (!integrity) return _match
            return `${prefix}${hrefAttr} integrity="${integrity}" crossorigin="anonymous"${midAttrs}${_relAttr}`
          },
        )
        // Also match when rel comes before href
        result = result.replace(
          /(<link\s[^>]*)(rel=["']stylesheet["'])([^>]*)(href=["']([^"']+)["'])/g,
          (_match, prefix, _relAttr, midAttrs, hrefAttr, url) => {
            const fileName = basename(url)
            const integrity = integrityMap.get(fileName)
            if (!integrity) return _match
            return `${prefix}${_relAttr}${midAttrs}${hrefAttr} integrity="${integrity}" crossorigin="anonymous"`
          },
        )

        return result
      })
    },
  }
}
