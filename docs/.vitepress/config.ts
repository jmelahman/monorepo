import { defineConfig } from 'vitepress'
import llmstxt from 'vitepress-plugin-llms'

export default defineConfig({
  vite: {
    plugins: [llmstxt()],
  },
  transformHead({ pageData, siteData }) {
    if (pageData.frontmatter.layout === 'home') return []
    return [
      ['link', {
        rel: 'alternate',
        type: 'text/markdown',
        href: `${siteData.base}${pageData.relativePath}`,
      }],
    ]
  },
  title: 'local-preview',
  description: 'Local-first preview-deployment orchestrator: a preview per commit.',
  base: '/local-preview/',
  lastUpdated: true,
  cleanUrls: true,
  ignoreDeadLinks: 'localhostLinks',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/local-preview/favicon.svg' }],
    ['meta', { name: 'theme-color', content: '#ef4444' }],
  ],
  themeConfig: {
    logo: '/favicon.svg',
    nav: [
      { text: 'Guide', link: '/guide/', activeMatch: '/guide/' },
      { text: 'Reference', link: '/reference/api', activeMatch: '/reference/' },
      { text: 'Releases', link: 'https://github.com/jmelahman/local-preview/releases' },
    ],
    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Introduction', link: '/guide/' },
            { text: 'Install', link: '/guide/install' },
            { text: 'Quickstart', link: '/guide/quickstart' },
            { text: 'Concepts', link: '/guide/concepts' },
            { text: 'Deployment triggers', link: '/guide/triggers' },
            { text: 'Uploading prebuilt artifacts', link: '/guide/uploads' },
            { text: 'Authentication (SSO)', link: '/guide/sso' },
            { text: 'Configuration', link: '/guide/configuration' },
            { text: 'Deploying to a server', link: '/guide/deploy-terraform' },
            { text: 'Worker-tier architecture', link: '/guide/worker-tier' },
            { text: 'External dependencies', link: '/guide/external-dependencies' },
            { text: 'Embedding', link: '/guide/embedding' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'REST API', link: '/reference/api' },
            { text: 'CLI', link: '/reference/cli' },
            { text: 'preview.toml', link: '/reference/preview-toml' },
          ],
        },
      ],
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/jmelahman/local-preview' },
    ],
    editLink: {
      pattern: 'https://github.com/jmelahman/local-preview/edit/master/docs/:path',
      text: 'Edit this page on GitHub',
    },
    search: { provider: 'local' },
    footer: {
      message: '<a href="https://jamison.lahman.dev/">Jamison Lahman</a>',
    },
  },
})
