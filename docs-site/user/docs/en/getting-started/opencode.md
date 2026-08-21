# OpenCode

> How to use CubeRouter in OpenCode

OpenCode is both a CLI + TUI AI coding agent that runs in the terminal and an IDE plugin integration, capable of fast code generation, debugging, project analysis, file operations, and cross-project collaboration across different development environments.

## Step 1: Install OpenCode

The simplest way to install OpenCode is the official install script:

```bash
curl -fsSL https://opencode.ai/install | bash
```

You can also install it with npm:

```bash
npm install -g opencode-ai
```

## Step 2: Get Your CubeRouter API Key and Model ID

1. **Register an account**: visit the CubeRouter platform, register an account and sign in (see [Register & Sign In](./register-login.md))
2. **Get an API key**: create a new API key on the **API Keys** page (see [Quick Start](./quick-start.md#step-2-create-an-api-key))
3. **Get a model ID**: on the **Model Square** page, use the **Group** filter to find the models available to your key's group, and copy the model ID for configuration

## Step 3: Configure OpenCode

::: warning Important
macOS & Linux & Windows are all supported — note that the config file path differs between systems. Also make sure the JSON file you edit stays valid (e.g. don't add or drop a stray `,`).
:::

Edit or create the `settings.json` file (macOS & Linux: `~/.config/opencode/opencode.json`, Windows: `user-profile-dir\.config\opencode\opencode.json`) and configure a custom provider:

```jsonc
// Replace your_platform_api_key with the API key you obtained
// Replace your_model_id with the model ID you obtained
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "cuberouter": {
      "options": {
        "baseURL": "https://your-platform.com/v1",
        "apiKey": "your_platform_api_key"
      },
      "models": {
        "your_model_id": {}
      }
    }
  },
  "model": "cuberouter/your_model_id"
}
```

For example, if the API key is `sk-APIKEY123abc456def789` and the model ID is `glm-5`, the config is:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "cuberouter": {
      "options": {
        "baseURL": "https://your-platform.com/v1",
        "apiKey": "sk-APIKEY123abc456def789"
      },
      "models": {
        "glm-5": {}
      }
    }
  },
  "model": "cuberouter/glm-5"
}
```

## Step 4: Start Using OpenCode

Once configured, go to your code working directory and run the `opencode` command in the terminal to start using OpenCode. Select the CubeRouter provider and the corresponding model, and you can start developing.
