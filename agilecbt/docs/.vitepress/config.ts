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
  title: 'AgileCBT',
  description: 'Agile planning meets CBT: check-ins, a gentle board, and an AI coach.',
  base: '/AgileCBT/',
  lastUpdated: true,
  cleanUrls: true,
  ignoreDeadLinks: 'localhostLinks',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/AgileCBT/favicon.svg' }],
    ['meta', { name: 'theme-color', content: '#c2703a' }],
  ],
  themeConfig: {
    logo: '/favicon.svg',
    nav: [
      { text: 'Guide', link: '/guide/', activeMatch: '/guide/' },
      { text: 'Reference', link: '/reference/api', activeMatch: '/reference/' },
      { text: 'Releases', link: 'https://github.com/jmelahman/AgileCBT/releases' },
    ],
    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Introduction', link: '/guide/' },
            { text: 'Install', link: '/guide/install' },
            { text: 'Quickstart', link: '/guide/quickstart' },
            { text: 'Configuration', link: '/guide/configuration' },
            { text: 'AI coach & MCP', link: '/guide/ai' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'REST API', link: '/reference/api' },
            { text: 'CLI', link: '/reference/cli' },
          ],
        },
      ],
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/jmelahman/AgileCBT' },
    ],
    editLink: {
      pattern: 'https://github.com/jmelahman/AgileCBT/edit/master/docs/:path',
      text: 'Edit this page on GitHub',
    },
    search: { provider: 'local' },
    footer: {
      message: '<a href="https://jamison.lahman.dev/">Jamison Lahman</a>',
    },
  },
})
