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
  title: 'Agentic Kanban',
  description: 'A kanban board for managing AI agent sessions.',
  base: '/agentic-kanban/',
  lastUpdated: true,
  cleanUrls: true,
  ignoreDeadLinks: 'localhostLinks',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/agentic-kanban/favicon.svg' }],
    ['meta', { name: 'theme-color', content: '#ef4444' }],
  ],
  themeConfig: {
    logo: '/favicon.svg',
    nav: [
      { text: 'Guide', link: '/guide/', activeMatch: '/guide/' },
      { text: 'Reference', link: '/reference/api', activeMatch: '/reference/' },
      { text: 'Releases', link: 'https://github.com/jmelahman/agentic-kanban/releases' },
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
            { text: 'Previews', link: '/guide/previews' },
            { text: 'Observability', link: '/guide/observability' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'REST API', link: '/reference/api' },
            { text: 'CLI', link: '/reference/cli' },
            { text: 'MCP', link: '/reference/mcp' },
          ],
        },
      ],
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/jmelahman/agentic-kanban' },
    ],
    editLink: {
      pattern: 'https://github.com/jmelahman/agentic-kanban/edit/master/docs/:path',
      text: 'Edit this page on GitHub',
    },
    search: { provider: 'local' },
    footer: {
      message: '<a href="https://jamison.lahman.dev/">Jamison Lahman</a>',
    },
  },
})
