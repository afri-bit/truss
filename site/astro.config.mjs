import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { trussDark, trussLight } from './src/styles/code-themes.mjs';

// Where the site is served. GitHub Pages serves a project at /<repo>/; set SITE_URL and SITE_BASE
// for another host (for example SITE_BASE=/ on a domain of its own).
const site = process.env.SITE_URL || 'https://afri-bit.github.io';
const base = process.env.SITE_BASE || '/truss';

export default defineConfig({
  site,
  base,
  integrations: [
    starlight({
      title: 'TRUSS',
      description:
        'TRUSS runs pipelines on your own machine: a command line, a local daemon with scheduling, and TRUSS Desktop.',
      logo: { src: './src/assets/logo.svg', alt: 'TRUSS' },
      favicon: '/favicon.svg',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/afri-bit/truss' }],
      // The brand (colours, type) is src/styles/brand.css; it replaces Starlight's default theme.
      customCss: ['./src/styles/fonts.css', './src/styles/brand.css'],
      components: { Footer: './src/components/DocsFooter.astro' },
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
      lastUpdated: false,
      // Code blocks: the brand's own syntax themes (no violet), in Starlight's frame.
      expressiveCode: { themes: [trussDark, trussLight], styleOverrides: { borderRadius: '6px' } },
      head: [
        { tag: 'meta', attrs: { name: 'theme-color', content: '#0c0e12' } },
      ],
      sidebar: [
        { label: 'Start here', items: [{ slug: 'docs/getting-started' }] },
        {
          label: 'Reference',
          items: [
            { slug: 'docs/pipeline-reference' },
            { slug: 'docs/cli' },
            { slug: 'docs/daemon' },
            { slug: 'docs/desktop' },
          ],
        },
        {
          label: 'Concepts',
          items: [{ slug: 'docs/architecture' }, { slug: 'docs/glossary' }],
        },
      ],
    }),
  ],
});
