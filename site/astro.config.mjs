import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

// Served from the custom domain vcode.io at the site root.
export default defineConfig({
  site: 'https://vcode.io',
  build: { assets: 'static' },
  integrations: [sitemap()],
});
