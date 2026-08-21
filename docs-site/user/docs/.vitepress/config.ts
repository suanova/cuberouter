import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'CubeRouter 用户文档',
  description: 'CubeRouter 用户文档',
  lang: 'zh-CN',
  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      themeConfig: {
        nav: [
          { text: '首页', link: '/' },
          { text: '快速开始', link: '/getting-started/quick-start' },
          { text: '使用指南', link: '/guide/dashboard' },
          { text: '支持', link: '/support/faq' }
        ],
        sidebar: {
          '/getting-started/': [
            {
              text: '快速开始',
              items: [
                { text: '快速上手', link: '/getting-started/quick-start' },
                { text: '注册与登录', link: '/getting-started/register-login' },
                { text: '界面概览', link: '/getting-started/ui-overview' },
                { text: 'Claude Code', link: '/getting-started/claude-code' },
                { text: 'OpenCode', link: '/getting-started/opencode' },
                { text: 'OpenClaw', link: '/getting-started/openclaw' }
              ]
            }
          ],
          '/guide/': [
            {
              text: '使用指南',
              items: [
                { text: '概览与数据看板', link: '/guide/dashboard' },
                { text: '模型广场', link: '/guide/models-market' },
                { text: 'API 密钥', link: '/guide/token' },
                { text: '配额与充值', link: '/guide/wallet' },
                { text: '订阅计划', link: '/guide/subscription' },
                { text: '游乐场', link: '/guide/playground' },
                { text: '使用日志', link: '/guide/log' },
                { text: '任务日志', link: '/guide/task' },
                { text: '个人设置', link: '/guide/personal-setting' },
                { text: '定价说明', link: '/guide/pricing' }
              ]
            }
          ],
          '/support/': [
            {
              text: '支持',
              items: [
                { text: '常见问题', link: '/support/faq' },
                { text: '故障排除', link: '/support/troubleshooting' },
                { text: '联系我们', link: '/support/contact' },
                { text: '更新日志', link: '/support/release-notes' }
              ]
            }
          ]
        },
        footer: {
          copyright: 'Copyright 2026 CubeRouter'
        },
        search: {
          provider: 'local',
          options: {
            translations: {
              button: {
                buttonText: '搜索文档...',
                buttonAriaLabel: '搜索文档'
              },
              modal: {
                noResultsText: '无法找到相关结果',
                resetButtonTitle: '清除查询条件',
                footer: {
                  selectText: '选择',
                  navigateText: '切换',
                  closeText: '关闭'
                }
              }
            }
          }
        },
        outline: {
          level: [2, 3],
          label: '页面导航'
        },
        docFooter: {
          prev: '上一页',
          next: '下一页'
        },
        lastUpdated: {
          text: '最后更新于',
          formatOptions: {
            dateStyle: 'short',
            timeStyle: 'short'
          }
        },
        returnToTopLabel: '返回顶部',
        sidebarMenuLabel: '菜单',
        darkModeSwitchLabel: '主题',
        lightModeSwitchTitle: '切换到浅色模式',
        darkModeSwitchTitle: '切换到深色模式'
      }
    },
    en: {
      label: 'English',
      lang: 'en-US',
      title: 'CubeRouter User Docs',
      description: 'CubeRouter User Documentation',
      head: [
        ['meta', { name: 'og:title', content: 'CubeRouter User Documentation' }],
        ['meta', { name: 'og:site_name', content: 'CubeRouter' }]
      ],
      themeConfig: {
        nav: [
          { text: 'Home', link: '/en/' },
          { text: 'Quick Start', link: '/en/getting-started/quick-start' },
          { text: 'User Guide', link: '/en/guide/dashboard' },
          { text: 'Support', link: '/en/support/faq' }
        ],
        sidebar: {
          '/en/getting-started/': [
            {
              text: 'Getting Started',
              items: [
                { text: 'Quick Start', link: '/en/getting-started/quick-start' },
                { text: 'Register & Sign In', link: '/en/getting-started/register-login' },
                { text: 'Interface Overview', link: '/en/getting-started/ui-overview' },
                { text: 'Claude Code', link: '/en/getting-started/claude-code' },
                { text: 'OpenCode', link: '/en/getting-started/opencode' },
                { text: 'OpenClaw', link: '/en/getting-started/openclaw' }
              ]
            }
          ],
          '/en/guide/': [
            {
              text: 'User Guide',
              items: [
                { text: 'Overview & Dashboard', link: '/en/guide/dashboard' },
                { text: 'Model Square', link: '/en/guide/models-market' },
                { text: 'API Keys', link: '/en/guide/token' },
                { text: 'Quota & Top-Up', link: '/en/guide/wallet' },
                { text: 'Subscription Plans', link: '/en/guide/subscription' },
                { text: 'Playground', link: '/en/guide/playground' },
                { text: 'Usage Logs', link: '/en/guide/log' },
                { text: 'Task Logs', link: '/en/guide/task' },
                { text: 'Profile Settings', link: '/en/guide/personal-setting' },
                { text: 'Pricing', link: '/en/guide/pricing' }
              ]
            }
          ],
          '/en/support/': [
            {
              text: 'Support',
              items: [
                { text: 'FAQ', link: '/en/support/faq' },
                { text: 'Troubleshooting', link: '/en/support/troubleshooting' },
                { text: 'Contact Us', link: '/en/support/contact' },
                { text: 'Release Notes', link: '/en/support/release-notes' }
              ]
            }
          ]
        },
        footer: {
          copyright: 'Copyright 2026 CubeRouter'
        },
        search: {
          provider: 'local',
          options: {
            translations: {
              button: {
                buttonText: 'Search docs...',
                buttonAriaLabel: 'Search docs'
              },
              modal: {
                noResultsText: 'No results found',
                resetButtonTitle: 'Clear query',
                footer: {
                  selectText: 'Select',
                  navigateText: 'Navigate',
                  closeText: 'Close'
                }
              }
            }
          }
        },
        outline: {
          level: [2, 3],
          label: 'On this page'
        },
        docFooter: {
          prev: 'Previous',
          next: 'Next'
        },
        lastUpdated: {
          text: 'Last updated on',
          formatOptions: {
            dateStyle: 'short',
            timeStyle: 'short'
          }
        },
        returnToTopLabel: 'Back to top',
        sidebarMenuLabel: 'Menu',
        darkModeSwitchLabel: 'Theme',
        lightModeSwitchTitle: 'Switch to light theme',
        darkModeSwitchTitle: 'Switch to dark theme'
      }
    },
    'zh-Hant': {
      label: '繁體中文',
      lang: 'zh-Hant',
      title: 'CubeRouter 用戶文檔',
      description: 'CubeRouter 用戶文檔',
      head: [
        ['meta', { name: 'og:title', content: 'CubeRouter 用戶文檔' }],
        ['meta', { name: 'og:site_name', content: 'CubeRouter' }]
      ],
      themeConfig: {
        nav: [
          { text: '首頁', link: '/zh-Hant/' },
          { text: '快速開始', link: '/zh-Hant/getting-started/quick-start' },
          { text: '使用指南', link: '/zh-Hant/guide/dashboard' },
          { text: '支援', link: '/zh-Hant/support/faq' }
        ],
        sidebar: {
          '/zh-Hant/getting-started/': [
            {
              text: '快速開始',
              items: [
                { text: '快速開始', link: '/zh-Hant/getting-started/quick-start' },
                { text: '註冊與登入', link: '/zh-Hant/getting-started/register-login' },
                { text: '界面概覽', link: '/zh-Hant/getting-started/ui-overview' },
                { text: 'Claude Code', link: '/zh-Hant/getting-started/claude-code' },
                { text: 'OpenCode', link: '/zh-Hant/getting-started/opencode' },
                { text: 'OpenClaw', link: '/zh-Hant/getting-started/openclaw' }
              ]
            }
          ],
          '/zh-Hant/guide/': [
            {
              text: '使用指南',
              items: [
                { text: '概覽與數據看板', link: '/zh-Hant/guide/dashboard' },
                { text: '模型廣場', link: '/zh-Hant/guide/models-market' },
                { text: 'API 金鑰管理', link: '/zh-Hant/guide/token' },
                { text: '配額與充值', link: '/zh-Hant/guide/wallet' },
                { text: '訂閱計劃', link: '/zh-Hant/guide/subscription' },
                { text: '遊樂場', link: '/zh-Hant/guide/playground' },
                { text: '使用日誌', link: '/zh-Hant/guide/log' },
                { text: '任務日誌', link: '/zh-Hant/guide/task' },
                { text: '個人資料', link: '/zh-Hant/guide/personal-setting' },
                { text: '定價說明', link: '/zh-Hant/guide/pricing' }
              ]
            }
          ],
          '/zh-Hant/support/': [
            {
              text: '支援',
              items: [
                { text: '常見問題', link: '/zh-Hant/support/faq' },
                { text: '故障排除', link: '/zh-Hant/support/troubleshooting' },
                { text: '聯繫支援', link: '/zh-Hant/support/contact' },
                { text: '版本日誌', link: '/zh-Hant/support/release-notes' }
              ]
            }
          ]
        },
        footer: {
          copyright: 'Copyright 2026 CubeRouter'
        },
        search: {
          provider: 'local',
          options: {
            translations: {
              button: {
                buttonText: '搜尋文件...',
                buttonAriaLabel: '搜尋文件'
              },
              modal: {
                noResultsText: '找不到搜尋結果',
                resetButtonTitle: '清除搜尋條件',
                footer: {
                  selectText: '選取',
                  navigateText: '切換',
                  closeText: '關閉'
                }
              }
            }
          }
        },
        outline: {
          level: [2, 3],
          label: '頁面導覽'
        },
        docFooter: {
          prev: '上一頁',
          next: '下一頁'
        },
        lastUpdated: {
          text: '最後更新於',
          formatOptions: {
            dateStyle: 'short',
            timeStyle: 'short'
          }
        },
        returnToTopLabel: '返回頂部',
        sidebarMenuLabel: '選單',
        darkModeSwitchLabel: '主題',
        lightModeSwitchTitle: '切換至淺色模式',
        darkModeSwitchTitle: '切換至深色模式'
      }
    }
  },
  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
    ['link', { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap' }],
    ['meta', { name: 'theme-color', content: '#0E72BC' }],
    ['meta', { name: 'og:type', content: 'website' }],
    ['meta', { name: 'og:title', content: 'CubeRouter 用户文档' }],
    ['meta', { name: 'og:site_name', content: 'CubeRouter 用户文档' }],
  ],

  themeConfig: {
    logo: '/logo.png',
    externalLinkIcon: true
  }
})
