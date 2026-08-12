import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import { buildExerciseSidebar } from './exercises'

export default withMermaid(defineConfig({
  lang: 'zh-CN',
  title: 'GoCampus',
  description: '面向 AI 应用开发实习的 Go 学习与实战手册',
  cleanUrls: true,
  lastUpdated: true,

  mermaid: {
    theme: 'base',
    themeVariables: {
      primaryColor: '#e6fcf5',
      primaryBorderColor: '#087f5b',
      primaryTextColor: '#212529',
      secondaryColor: '#f1f3f5',
      secondaryBorderColor: '#099268',
      tertiaryColor: '#fff9db',
      tertiaryBorderColor: '#f59f00',
      lineColor: '#099268',
      textColor: '#343a40',
      fontSize: '14px',
      fontFamily: 'system-ui, -apple-system, sans-serif',
      nodeTextColor: '#212529',
    },
  },
  mermaidPlugin: {
    class: 'mermaid',
  },

  head: [
    ['meta', { name: 'theme-color', content: '#087f5b' }],
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }]
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'GoCampus',

    nav: [
      { text: '首页', link: '/' },
      {
        text: '学习计划',
        items: [
          { text: '总体规划', link: '/学习计划安排/总体规划' },
          { text: '第一阶段 · Go 语言深入', link: '/学习计划安排/第一阶段-Go语言深入' },
          { text: '第二阶段 · 计算机基础', link: '/学习计划安排/第二阶段-计算机基础强化' },
          { text: '第三阶段 · AI 应用开发', link: '/学习计划安排/第三阶段-AI应用开发基础' }
        ]
      },
      {
        text: '知识详解',
        items: [
          { text: '第一阶段 · Go 语言深入', link: '/第一阶段-知识点详解' },
          { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' },
          { text: 'Interface 底层原理', link: '/第一阶段-知识详解/Interface底层原理' },
          { text: 'String 与字节切片', link: '/第一阶段-知识详解/String与字节切片' },
          { text: 'Go 并发编程详解', link: '/第一阶段-知识详解/Go并发编程详解' },
          { text: 'Go 并发编程问答集', link: '/第一阶段-知识详解/Go并发编程问答集' },
          { text: 'Go Context 详解', link: '/第一阶段-知识详解/Go Context 详解' },
          { text: '第二阶段 · 操作系统', link: '/第二阶段-知识详解/操作系统面试详解' },
          { text: '第二阶段 · 计算机网络', link: '/第二阶段-知识详解/计算机网络面试详解' }
        ]
      },
      {
        text: '项目实战',
        items: [
          { text: 'RAG 文档问答系统', link: '/phase3/docs-rag/' },
          { text: 'AI Agent Harness', link: '/phase3/agent-harness/' },
          { text: 'pi 开源 Harness 拆解', link: '/phase3/pi-harness/' }
        ]
      },
      {
        text: '代码练习',
        items: [
          { text: '练习指南', link: '/练习指南' },
          { text: '习题集和答案', link: '/习题集和答案/' }
        ]
      }
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
        }
      ],
      '/第一阶段-知识': [
        {
          text: '第一阶段 · 知识详解',
          items: [
            { text: '知识点总览', link: '/第一阶段-知识点详解' },
            { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' },
            { text: 'Interface 底层原理', link: '/第一阶段-知识详解/Interface底层原理' },
            { text: 'String 与字节切片', link: '/第一阶段-知识详解/String与字节切片' },
            { text: 'Go 并发编程详解', link: '/第一阶段-知识详解/Go并发编程详解' },
            { text: 'Go 并发编程问答集', link: '/第一阶段-知识详解/Go并发编程问答集' },
            { text: 'Go Context 详解', link: '/第一阶段-知识详解/Go Context 详解' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第一阶段-Go语言深入' },
            { text: '代码练习指南', link: '/练习指南' }
          ]
        }
      ],
      '/练习指南': [
        {
          text: '代码练习',
          items: [
            { text: '练习指南', link: '/练习指南' },
            { text: '习题集和答案', link: '/习题集和答案/' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '知识点总览', link: '/第一阶段-知识点详解' },
            { text: '第一阶段学习计划', link: '/学习计划安排/第一阶段-Go语言深入' }
          ]
        }
      ],
      '/第二阶段-知识详解/': [
        {
          text: '第二阶段 · 计算机基础',
          items: [
            { text: '操作系统面试详解', link: '/第二阶段-知识详解/操作系统面试详解' },
            { text: '计算机网络面试详解', link: '/第二阶段-知识详解/计算机网络面试详解' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第二阶段-计算机基础强化' },
            { text: '代码练习指南', link: '/练习指南' }
          ]
        }
      ],
      '/习题集和答案/': buildExerciseSidebar(),
      '/phase3/agent-harness/': [
        {
          text: 'AI Agent Harness',
          items: [
            { text: '项目首页', link: '/phase3/agent-harness/' },
            { text: '学习笔记', link: '/phase3/agent-harness/学习笔记' },
            { text: '项目设计', link: '/phase3/agent-harness/项目设计' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第三阶段-AI应用开发基础' },
            { text: '第三阶段总览', link: '/phase3/' },
            { text: 'RAG 文档问答系统', link: '/phase3/docs-rag/' }
          ]
        }
      ],
      '/phase3/pi-harness/': [
        {
          text: 'pi 开源 Harness 拆解',
          items: [
            { text: '项目全景', link: '/phase3/pi-harness/' },
            { text: '01 核心机制：Agent Loop', link: '/phase3/pi-harness/核心机制' },
            { text: '02 LLM 抽象层', link: '/phase3/pi-harness/LLM抽象层' },
            { text: '03 工程化落地', link: '/phase3/pi-harness/工程化落地' },
            { text: '04 Go 落地与面试', link: '/phase3/pi-harness/Go落地与面试' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第三阶段-AI应用开发基础' },
            { text: '第三阶段总览', link: '/phase3/' },
            { text: 'AI Agent Harness 项目', link: '/phase3/agent-harness/' }
          ]
        }
      ],
      '/phase3/docs-rag/': [
        {
          text: 'RAG 文档问答系统',
          items: [
            { text: '项目首页', link: '/phase3/docs-rag/' },
            { text: '学习笔记', link: '/phase3/docs-rag/学习笔记' },
            { text: '项目设计', link: '/phase3/docs-rag/项目设计' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第三阶段-AI应用开发基础' },
            { text: '第三阶段总览', link: '/phase3/' }
          ]
        }
      ],
      '/phase3/': [
        {
          text: '第三阶段 · AI 应用开发',
          items: [
            { text: '阶段总览', link: '/phase3/' },
            { text: 'RAG 文档问答系统', link: '/phase3/docs-rag/' },
            { text: 'AI Agent Harness', link: '/phase3/agent-harness/' },
            { text: 'pi 开源 Harness 拆解', link: '/phase3/pi-harness/' }
          ]
        }
      ]
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
}))
