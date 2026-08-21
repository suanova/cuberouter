# Release Notes

This document records CubeRouter's version update history, new features, improvements, and known issues.

## Latest Version

### v1.0.0 (2026-03)

Initial release. CubeRouter is an enterprise-grade AI model aggregation and relay platform, providing a unified OpenAI-compatible API. It supports 40+ AI providers and enterprise private models, with intelligent routing, billing, and quota management.

#### Core features

- **Multi-model support**: models from 40+ AI providers including OpenAI, Claude, Gemini, DeepSeek, with support for connecting enterprise private models
- **Intelligent routing**: multi-channel weighted distribution, automatic failure retry, and user-level model rate limiting
- **Billing system**: pay-as-you-go and per-call billing, with differentiated pricing for cached / multimodal tokens
- **Quota & subscriptions**: a unified quota account, with online payment, redemption code top-ups, and subscription plans
- **Key management**: API keys support groups, model restrictions, IP whitelists, and expiration times
- **Usage & task logs**: per-call records and async task records for image / video, etc.
- **Data dashboard**: multi-dimensional statistics on usage, consumption, and performance
- **Multilingual interface**: supports 6 languages — zh / en / ja / fr / ru / vi
- **Security features**: two-factor authentication (2FA), Passkeys, and multi-method OAuth sign-in

#### Clients & tools

- Compatible with the OpenAI protocol; works with tools like Claude Code, opencode, and OpenClaw
- The in-app **Playground** supports online model testing and parameter debugging
