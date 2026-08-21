# FAQ

The following are the most frequently asked questions and answers to help you resolve common issues quickly.

## Q1: What is CubeRouter?

CubeRouter is an AI model aggregation and relay platform. It provides a unified OpenAI-compatible API gateway to access models from 40+ providers, including OpenAI GPT series, Anthropic Claude, Gemini, DeepSeek, and more.

## Q2: What's the difference between CubeRouter and using model vendor APIs directly?

The main differences:

- **Unified interface**: access models from multiple vendors through a single API address and one set of keys
- **Unified billing**: all calls are converted into quota — top up in one place, reconcile in one place
- **Multi-channel routing**: the same model can be backed by multiple upstream channels, with automatic load balancing and failure retry
- **Enterprise-grade features**: full user management, grouping, rate limiting, logging, and monitoring

## Q3: Which models are supported?

The platform supports models from 40+ providers; the specific available models and prices are shown in the **Model Square** in the platform.

## Q4: Which client tools are supported?

CubeRouter's API is compatible with the OpenAI protocol and can be used directly with Claude Code, opencode, OpenClaw, and all kinds of SDKs. See [Quick Start](../getting-started/quick-start.md).

## Q5: How is billing calculated?

There are two types: pay-as-you-go (billed by token) and per-call billing (a fixed price per call). See [Pricing Explained](../guide/pricing.md).

## Q6: What if I lose a key?

The full key is only displayed once at creation. If a key is lost, you can delete the old key and create a new one on the **API Keys** page; if you suspect a key has leaked, simply disable or delete it.

## Q7: How is data security guaranteed?

- HTTPS transport encryption is supported site-wide
- Keys support IP whitelist restrictions
- Self-hosted private deployment is supported — data stays local
- Two-factor authentication (2FA) and Passkeys are supported for sensitive personal operations

## Q8: What if the service responds slowly?

Troubleshooting steps:

1. Check your local network connection
2. Confirm the API Base URL is configured correctly (avoid extra `/v1` path segments)
3. Try switching models or retrying later
4. If the issue persists, contact the administrator with the request ID from your usage logs
