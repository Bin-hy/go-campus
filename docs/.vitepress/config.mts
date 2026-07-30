import { defineConfig } from 'vitepress'
import { buildExerciseSidebar } from './exercises'

export default defineConfig({
  lang: 'zh-CN',
  title: 'GoCampus',
  description: '面向 AI 应用开发实习的 Go 学习与实战手册',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['meta', { name: 'theme-color', content: '#087f5b' }],
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }]
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'GoCampus',

    nav: [
      { text: '首页', link: '/' },
      { text: '总体规划', link: '/学习计划安排/总体规划' },
      {
        text: '学习阶段',
        items: [
          { text: '第一阶段 · Go 语言深入', link: '/学习计划安排/第一阶段-Go语言深入' },
          { text: '第二阶段 · 计算机基础', link: '/学习计划安排/第二阶段-计算机基础强化' },
          { text: '第三阶段 · AI 应用开发', link: '/学习计划安排/第三阶段-AI应用开发基础' }
        ]
      },
      {
        text: '知识详解',
        items: [
          { text: '第一阶段 · 知识点总览', link: '/第一阶段-知识点详解' },
          { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' }
        ]
      },
      { text: '习题集和答案', link: '/习题集和答案/' },
      { text: '练习指南', link: '/练习指南' }
    ],

    sidebar: {
      '/学习计划安排/': [
        {
          text: '学习计划',
          items: [
            { text: '总体规划', link: '/学习计划安排/总体规划' },
            { text: '第一阶段：Go 语言深入', link: '/学习计划安排/第一阶段-Go语言深入' },
            { text: '第二阶段：计算机基础强化', link: '/学习计划安排/第二阶段-计算机基础强化' },
            { text: '第三阶段：AI 应用开发基础', link: '/学习计划安排/第三阶段-AI应用开发基础' }
          ]
        },
        {
          text: '配套内容',
          items: [
            { text: '第一阶段知识详解', link: '/第一阶段-知识点详解' },
            { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' },
            { text: '代码练习指南', link: '/练习指南' }
          ]
        }
      ],
      '/第一阶段-知识点详解': [
        {
          text: 'Go 语言深入',
          items: [
            { text: '知识点详解', link: '/第一阶段-知识点详解' },
            { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' },
            { text: '阶段学习计划', link: '/学习计划安排/第一阶段-Go语言深入' },
            { text: '代码练习指南', link: '/练习指南' }
          ]
        }
      ],
      '/第一阶段-知识详解/': [
        {
          text: '第一阶段知识详解',
          items: [
            { text: '知识点总览', link: '/第一阶段-知识点详解' },
            { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' },
            { text: '阶段学习计划', link: '/学习计划安排/第一阶段-Go语言深入' },
            { text: '代码练习指南', link: '/练习指南' }
          ]
        }
      ],
      '/练习指南': [
        {
          text: '实践训练',
          items: [
            { text: '代码练习指南', link: '/练习指南' },
            { text: '第一阶段知识详解', link: '/第一阶段-知识点详解' }
          ]
        }
      ],
      '/习题集和答案/': buildExerciseSidebar()
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/Bin-hy/go-campus' }
    ],

    search: {
      provider: 'local',
      options: {
        translations: {
          button: {
            buttonText: '搜索文档',
            buttonAriaLabel: '搜索文档'
          },
          modal: {
            noResultsText: '没有找到相关结果',
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
      label: '本页目录'
    },
    docFooter: {
      prev: '上一篇',
      next: '下一篇'
    },
    lastUpdated: {
      text: '最后更新于',
      formatOptions: {
        dateStyle: 'medium',
        timeStyle: 'short'
      }
    },
    returnToTopLabel: '返回顶部',
    sidebarMenuLabel: '目录',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    footer: {
      message: '持续学习，持续构建。',
      copyright: 'GoCampus Learning Notes'
    }
  }
})
