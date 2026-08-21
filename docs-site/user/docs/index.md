---
layout: home
title: CubeRouter 文档中心
titleTemplate: CubeRouter 用户文档

hero:
  name: CubeRouter
  text: 用户文档指南
  tagline: 快速上手使用 CubeRouter AI 网关服务，一站式接入主流 AI 模型
  actions:
    - theme: brand
      text: 用户指南
      link: /guide/dashboard
    - theme: alt
      text: 快速上手
      link: /getting-started/quick-start

features:
  - icon: 💼
    title: 快速开始
    details: 帮助您快速上手 CubeRouter，从注册账号、创建 API 密钥到发起第一次 AI 调用，只需几分钟即可完成。
    link: /getting-started/quick-start
  - icon: 🧭
    title: 使用指南
    details: 全面了解 CubeRouter 的各项功能：模型广场、API 密钥管理、钱包充值、订阅计划、游乐场、使用与任务日志。
    link: /guide/dashboard
  - icon: 🛟
    title: 支持
    details: 常见问题、故障排除与更新日志，遇到问题时随时查阅，也可随时联系我们。
    link: /support/faq
---

<style>
:root {
  --vp-home-hero-name-color: transparent;
  --vp-home-hero-name-background: -webkit-linear-gradient(120deg, #0E72BC 30%, #3EA4EC);
  --vp-home-hero-image-background-image: linear-gradient(-45deg, #0E72BC 50%, #3EA4EC 50%);
  --vp-home-hero-image-filter: blur(44px);
}

.dark {
  --vp-home-hero-image-background-image: linear-gradient(-45deg, #3EA4EC 50%, #0E72BC 50%);
}

@media (min-width: 640px) {
  :root {
    --vp-home-hero-image-filter: blur(56px);
  }
}

@media (min-width: 960px) {
  :root {
    --vp-home-hero-image-filter: blur(68px);
  }
}

.VPHero .name {
  font-weight: 700;
}

.VPHero .tagline {
  color: var(--vp-c-text-2);
  font-size: 1.25rem;
}

.VPFeature {
  transition: all 0.3s ease;
  border-radius: 12px;
  border: 1px solid var(--vp-c-border);
}

.VPFeature:hover {
  border-color: var(--vp-c-brand-1);
  box-shadow: 0 8px 24px rgba(14, 114, 188, 0.15);
  transform: translateY(-4px);
}

.dark .VPFeature:hover {
  box-shadow: 0 8px 24px rgba(62, 164, 236, 0.15);
}

.VPFeature .icon {
  font-size: 1.5rem;
}

.VPFeature .title {
  font-weight: 600;
}

.VPFeature .details {
  color: var(--vp-c-text-2);
  line-height: 1.6;
}
</style>
