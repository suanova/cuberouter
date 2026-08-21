---
layout: home
title: CubeRouter User Documentation
titleTemplate: CubeRouter User Docs

hero:
  name: CubeRouter
  text: User Documentation Guide
  tagline: Get started with the CubeRouter AI gateway in minutes — one platform for all major AI models
  actions:
    - theme: brand
      text: User Guide
      link: /en/guide/dashboard
    - theme: alt
      text: Quick Start
      link: /en/getting-started/quick-start

features:
  - icon: 💼
    title: Quick Start
    details: Get up and running with CubeRouter quickly — from account registration and API key creation to your first AI call, in just a few minutes.
    link: /en/getting-started/quick-start
  - icon: 🧭
    title: User Guide
    details: "Explore all of CubeRouter: Model Square, API key management, wallet & top-up, subscription plans, Playground, and usage & task logs."
    link: /en/guide/dashboard
  - icon: 🛟
    title: Support
    details: FAQs, troubleshooting, and release notes. Look them up any time you hit a snag, or just reach out to us directly.
    link: /en/support/faq
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
