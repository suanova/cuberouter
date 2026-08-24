---
layout: home
title: CubeRouter 管理员文档
titleTemplate: CubeRouter 管理员文档

hero:
  name: CubeRouter
  text: AI 网关管理平台
  tagline: 统一管理 40+ AI 服务提供商，简化 AI 模型接入与运维
  actions:
    - theme: brand
      text: 快速入门
      link: /guide/channel/index
    - theme: alt
      text: 系统设置
      link: /guide/system/index

features:
  - icon: 📡
    title: 渠道管理
    details: 配置和管理 AI 模型提供商的 API 接入渠道，支持 40+ 种服务提供商，实现负载均衡与故障转移。
    link: /guide/channel/index
  - icon: 🧩
    title: 渠道高级设置
    details: 模型映射、优先级与权重、自动禁用、请求头与参数覆盖，精细控制渠道行为。
    link: /guide/channel/advanced
  - icon: 🤖
    title: 模型管理
    details: 管理平台支持的 AI 模型，包括启用状态、定价、用户组权限与上游同步。
    link: /guide/model/index
  - icon: 📂
    title: 分组管理
    details: 通过用户组隔离渠道访问权限与计费倍率，支持令牌自动分组容灾。
    link: /guide/group/index
  - icon: 👥
    title: 用户管理
    details: 管理用户账户、角色权限、额度分配，支持多种登录方式与邀请系统。
    link: /guide/user/index
  - icon: 🎫
    title: 兑换码管理
    details: 批量生成兑换码，支持有效期管理和使用状态追踪。
    link: /guide/redemption/index
  - icon: 💳
    title: 订阅管理
    details: 灵活配置订阅方案，支持有效期与额度设置，集成多种支付方式。
    link: /guide/subscription/index
  - icon: 📊
    title: 日志与统计
    details: 查看全平台 API 调用记录和消耗统计，支持多维度分析。
    link: /guide/log/index
  - icon: ⚙️
    title: 系统设置
    details: 配置运营参数、计费倍率、支付集成、性能监控与限流策略。
    link: /guide/system/index
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
