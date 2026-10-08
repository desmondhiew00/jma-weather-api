import { rename, rm } from 'node:fs/promises';
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';
import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';

// Static output: the site is HTML plus a little client JS that talks straight to
// api.tenkinow.com, so there is no server to deploy alongside it.
export default defineConfig({
  site: 'https://tenkinow.com',
  // English at the root, Japanese under /ja. The sitemap emits hreflang pairs
  // from this, matching the <link rel="alternate"> tags in the layout.
  i18n: {
    locales: ['en', 'ja'],
    defaultLocale: 'en',
    routing: { prefixDefaultLocale: false },
  },
  vite: { plugins: [tailwindcss()] },
  integrations: [
    // Astro writes the root 404 as dist/404.html but a nested one as
    // dist/ja/404/index.html. Cloudflare Pages only looks for 404.html files,
    // walking up the directory tree, so the Japanese one has to sit at
    // dist/ja/404.html or a bad /ja/* path falls through to the English 404.
    {
      name: 'ja-404-flat',
      hooks: {
        'astro:build:done': async ({ dir }) => {
          const from = new URL('ja/404/index.html', dir);
          await rename(from, new URL('ja/404.html', dir));
          await rm(new URL('ja/404/', dir), { recursive: true });
        },
      },
    },
    react(),
    sitemap({
      // The 404 is noindex; listing it in the sitemap asks a crawler to index the
      // one page that says it should not be.
      filter: (page) => !page.endsWith('/404/') && !page.endsWith('/404'),
      i18n: { defaultLocale: 'en', locales: { en: 'en', ja: 'ja' } },
    }),
  ],
});
