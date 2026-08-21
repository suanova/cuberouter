---
layout: home
title: CubeRouter 文檔中心
titleTemplate: CubeRouter 用戶文檔

hero:
  name: CubeRouter
  text: 用戶文檔指南
  tagline: 快速上手使用 CubeRouter AI 網關服務，一站式接入主流 AI 模型
  actions:
    - theme: brand
      text: 用戶指南
      link: /zh-Hant/guide/dashboard
    - theme: alt
      text: 快速上手
      link: /zh-Hant/getting-started/quick-start

features:
  - icon: 💼
    title: 快速開始
    details: 幫助您快速上手 CubeRouter，從註冊賬號、創建 API 金鑰到發起第一次 AI 調用，只需幾分鐘即可完成。
    link: /zh-Hant/getting-started/quick-start
  - icon: 🧭
    title: 使用指南
    details: 全面瞭解 CubeRouter 的各項功能：模型廣場、API 金鑰管理、錢包充值、訂閱計劃、遊樂場、使用與任務日誌。
    link: /zh-Hant/guide/dashboard
  - icon: 🛟
    title: 支援
    details: 常見問題、故障排除與更新日誌，遇到問題時隨時查閱，也可隨時聯繫我們。
    link: /zh-Hant/support/faq
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
