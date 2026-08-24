---
layout: home
title: CubeRouter Admin Documentation
titleTemplate: CubeRouter Admin Docs

hero:
  name: CubeRouter
  text: AI Gateway Admin Platform
  tagline: Manage 40+ AI service providers from one platform — simplify AI model integration and operations
  actions:
    - theme: brand
      text: Quick Start
      link: /en/guide/channel/index
    - theme: alt
      text: System Settings
      link: /en/guide/system/index

features:
  - icon: 📡
    title: Channel Management
    details: Configure and manage API channels for AI providers, supporting 40+ providers with load balancing and failover.
    link: /en/guide/channel/index
  - icon: 🧩
    title: Channel Advanced
    details: Model mapping, priority & weight, auto-disable, request header and parameter overrides for fine-grained channel control.
    link: /en/guide/channel/advanced
  - icon: 🤖
    title: Model Management
    details: Manage model metadata and pricing, visibility and group permissions, with upstream synchronization.
    link: /en/guide/model/index
  - icon: 📂
    title: Group Management
    details: Isolate channel access and billing ratios per group, with automatic token group failover.
    link: /en/guide/group/index
  - icon: 👥
    title: User Management
    details: Manage user accounts, roles, quota allocation, with multiple login methods and an invite system.
    link: /en/guide/user/index
  - icon: 🎫
    title: Redemption Codes
    details: Generate redemption codes in batches, with expiry management and usage tracking.
    link: /en/guide/redemption/index
  - icon: 💳
    title: Subscription Management
    details: Flexible subscription plans with validity and quota settings, integrated with multiple payment methods.
    link: /en/guide/subscription/index
  - icon: 📊
    title: Logs & Statistics
    details: Review platform-wide API calls and quota consumption with multi-dimensional analysis.
    link: /en/guide/log/index
  - icon: ⚙️
    title: System Settings
    details: Configure operation parameters, billing ratios, payment integration, performance monitoring and rate limits.
    link: /en/guide/system/index
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
