import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'CubeRouter 管理员文档',
  description: 'CubeRouter AI 网关平台管理员文档中心',
  lang: 'zh-CN',
  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      themeConfig: {
        nav: [
          { text: '首页', link: '/' },
          { text: '渠道管理', link: '/guide/channel/index' },
          { text: '订阅管理', link: '/guide/subscription/index' },
          { text: '系统设置', link: '/guide/system/index' }
        ],
        sidebar: {
          '/guide/': [
            {
              text: '渠道管理',
              items: [
                { text: '渠道配置', link: '/guide/channel/index' },
                { text: '高级设置', link: '/guide/channel/advanced' }
              ]
            },
            {
              text: '模型管理',
              items: [
                { text: '模型配置', link: '/guide/model/index' }
              ]
            },
            {
              text: '分组管理',
              items: [
                { text: '分组配置', link: '/guide/group/index' }
              ]
            },
            {
              text: '用户管理',
              items: [
                { text: '用户管理', link: '/guide/user/index' }
              ]
            },
            {
              text: '兑换码管理',
              items: [
                { text: '兑换码配置', link: '/guide/redemption/index' }
              ]
            },
            {
              text: '订阅管理',
              items: [
                { text: '订阅方案配置', link: '/guide/subscription/index' }
              ]
            },
            {
              text: '日志与统计',
              items: [
                { text: '日志与统计', link: '/guide/log/index' }
              ]
            },
            {
              text: '系统设置',
              items: [
                { text: '系统设置', link: '/guide/system/index' }
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
      title: 'CubeRouter Admin Documentation',
      description: 'CubeRouter AI Gateway Admin Documentation',
      head: [
        ['meta', { name: 'og:title', content: 'CubeRouter Admin Documentation' }],
        ['meta', { name: 'og:site_name', content: 'CubeRouter' }]
      ],
      themeConfig: {
        nav: [
          { text: 'Home', link: '/en/' },
          { text: 'Channels', link: '/en/guide/channel/index' },
          { text: 'Subscriptions', link: '/en/guide/subscription/index' },
          { text: 'System Settings', link: '/en/guide/system/index' }
        ],
        sidebar: {
          '/en/guide/': [
            {
              text: 'Channel Management',
              items: [
                { text: 'Channel Setup', link: '/en/guide/channel/index' },
                { text: 'Advanced Settings', link: '/en/guide/channel/advanced' }
              ]
            },
            {
              text: 'Model Management',
              items: [
                { text: 'Model Setup', link: '/en/guide/model/index' }
              ]
            },
            {
              text: 'Group Management',
              items: [
                { text: 'Group Setup', link: '/en/guide/group/index' }
              ]
            },
            {
              text: 'User Management',
              items: [
                { text: 'User Management', link: '/en/guide/user/index' }
              ]
            },
            {
              text: 'Redemption Codes',
              items: [
                { text: 'Redemption Codes', link: '/en/guide/redemption/index' }
              ]
            },
            {
              text: 'Subscription Management',
              items: [
                { text: 'Subscription Plans', link: '/en/guide/subscription/index' }
              ]
            },
            {
              text: 'Logs & Statistics',
              items: [
                { text: 'Logs & Statistics', link: '/en/guide/log/index' }
              ]
            },
            {
              text: 'System Settings',
              items: [
                { text: 'System Settings', link: '/en/guide/system/index' }
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
      title: 'CubeRouter 管理員文件',
      description: 'CubeRouter AI 網關平臺管理員文件中心',
      head: [
        ['meta', { name: 'og:title', content: 'CubeRouter 管理員文件' }],
        ['meta', { name: 'og:site_name', content: 'CubeRouter' }]
      ],
      themeConfig: {
        nav: [
          { text: '首頁', link: '/zh-Hant/' },
          { text: '渠道管理', link: '/zh-Hant/guide/channel/index' },
          { text: '訂閱管理', link: '/zh-Hant/guide/subscription/index' },
          { text: '系統設置', link: '/zh-Hant/guide/system/index' }
        ],
        sidebar: {
          '/zh-Hant/guide/': [
            {
              text: '渠道管理',
              items: [
                { text: '渠道配置', link: '/zh-Hant/guide/channel/index' },
                { text: '高級設置', link: '/zh-Hant/guide/channel/advanced' }
              ]
            },
            {
              text: '模型管理',
              items: [
                { text: '模型配置', link: '/zh-Hant/guide/model/index' }
              ]
            },
            {
              text: '分組管理',
              items: [
                { text: '分組配置', link: '/zh-Hant/guide/group/index' }
              ]
            },
            {
              text: '用戶管理',
              items: [
                { text: '用戶管理', link: '/zh-Hant/guide/user/index' }
              ]
            },
            {
              text: '兌換碼管理',
              items: [
                { text: '兌換碼配置', link: '/zh-Hant/guide/redemption/index' }
              ]
            },
            {
              text: '訂閱管理',
              items: [
                { text: '訂閱方案配置', link: '/zh-Hant/guide/subscription/index' }
              ]
            },
            {
              text: '日誌與統計',
              items: [
                { text: '日誌與統計', link: '/zh-Hant/guide/log/index' }
              ]
            },
            {
              text: '系統設置',
              items: [
                { text: '系統設置', link: '/zh-Hant/guide/system/index' }
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
    ['meta', { name: 'og:title', content: 'CubeRouter 管理员文档' }],
    ['meta', { name: 'og:site_name', content: 'CubeRouter 管理员文档' }],
  ],

  themeConfig: {
    logo: '/logo.png',
    externalLinkIcon: true
  }
})
