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
  title: 'Fullstack Template',
  description: 'A template for fullstack Go + React applications.',
  base: '/fullstack-template/',
  lastUpdated: true,
  cleanUrls: true,
  ignoreDeadLinks: 'localhostLinks',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/fullstack-template/favicon.svg' }],
    ['meta', { name: 'theme-color', content: '#3b82f6' }],
  ],
  themeConfig: {
    logo: '/favicon.svg',
    nav: [
      { text: 'Guide', link: '/guide/', activeMatch: '/guide/' },
      { text: 'Reference', link: '/reference/api', activeMatch: '/reference/' },
      { text: 'Releases', link: 'https://github.com/jmelahman/fullstack-template/releases' },
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
      { icon: 'github', link: 'https://github.com/jmelahman/fullstack-template' },
    ],
    editLink: {
      pattern: 'https://github.com/jmelahman/fullstack-template/edit/master/docs/:path',
      text: 'Edit this page on GitHub',
    },
    search: { provider: 'local' },
    footer: {
      message: '<a href="https://jamison.lahman.dev/">Jamison Lahman</a>',
    },
  },
})
