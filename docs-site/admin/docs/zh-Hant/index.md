---
layout: home
title: CubeRouter 管理員文件
titleTemplate: CubeRouter 管理員文件

hero:
  name: CubeRouter
  text: AI 網關管理平臺
  tagline: 統一管理 40+ AI 服務提供商，簡化 AI 模型接入與運維
  actions:
    - theme: brand
      text: 快速入門
      link: /zh-Hant/guide/channel/index
    - theme: alt
      text: 系統設置
      link: /zh-Hant/guide/system/index

features:
  - icon: 📡
    title: 渠道管理
    details: 配置和管理 AI 模型提供商的 API 接入渠道，支援 40+ 種服務提供商，實現負載均衡與故障轉移。
    link: /zh-Hant/guide/channel/index
  - icon: 🧩
    title: 渠道高級設置
    details: 模型映射、優先級與權重、自動禁用、請求頭與參數覆蓋，精細控制渠道行為。
    link: /zh-Hant/guide/channel/advanced
  - icon: 🤖
    title: 模型管理
    details: 管理平臺支援的 AI 模型，包括啟用狀態、定價、用戶組權限與上游同步。
    link: /zh-Hant/guide/model/index
  - icon: 📂
    title: 分組管理
    details: 通過用戶組隔離渠道訪問權限與計費倍率，支援令牌自動分組容災。
    link: /zh-Hant/guide/group/index
  - icon: 👥
    title: 用戶管理
    details: 管理用戶賬戶、角色權限、額度分配，支援多種登錄方式與邀請系統。
    link: /zh-Hant/guide/user/index
  - icon: 🎫
    title: 兌換碼管理
    details: 批量生成兌換碼，支援有效期管理和使用狀態追蹤。
    link: /zh-Hant/guide/redemption/index
  - icon: 💳
    title: 訂閱管理
    details: 靈活配置訂閱方案，支援有效期與額度設置，集成多種支付方式。
    link: /zh-Hant/guide/subscription/index
  - icon: 📊
    title: 日誌與統計
    details: 查看全平臺 API 調用記錄和消耗統計，支援多維度分析。
    link: /zh-Hant/guide/log/index
  - icon: ⚙️
    title: 系統設置
    details: 配置運營參數、計費倍率、支付集成、性能監控與限流策略。
    link: /zh-Hant/guide/system/index
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
