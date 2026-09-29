# Codex

> How to use CubeRouter in Codex

Codex is an intelligent coding tool that runs in the terminal. It interacts through natural-language commands and helps developers quickly complete code generation, debugging, refactoring, and more.

## Step 1: Install Codex

Prerequisites:

- You need [Node.js 18 or newer](https://nodejs.org/en/download/)
- On macOS, we recommend installing Node.js via [nvm](https://github.com/nvm-sh/nvm) or [Homebrew](https://formulae.brew.sh/formula/node). Installing from a package installer is not recommended (you may run into permission issues later)
- Windows users also need [Git for Windows](https://git-scm.com/install/windows)

Open a terminal and install Codex:

```bash
npm install -g @openai/codex
```

Run the following command to verify the installation — a version number means it installed successfully:

```bash
codex --version
```

## Step 2: Get Your CubeRouter API Key and Model ID

1. **Register an account**: visit the CubeRouter platform, register an account and sign in (see [Register & Sign In](./register-login.md))
2. **Get an API key**: create a new API key on the **API Keys** page (see [Quick Start](./quick-start.md#step-2-create-an-api-key))
3. **Get a model ID**: on the **Model Square** page, use the **Endpoint Type** filter to find the models that support Response, and copy the model ID for configuration

## Step 3: Configure Codex

::: warning Important
macOS & Linux & Windows are all supported — note that the config file path differs between systems.
:::

Edit or create the `config.toml` file (macOS & Linux: `~/.codex/config.toml`; Windows: `user-profile-dir/.codex/config.toml`)

```toml
model_provider = "cuberouter"
model = "<you_selected_model_id>"  # e.g. gpt-5.6-terra

[model_providers.cuberouter]
name = "CubeRouter"
base_url = "<base_url>" # e.g. https://cuberouter.cn or https://cuberouter.com  
env_key = "CODEX_CUBEROUTER_API_KEY"
wire_api = "responses"

```

## Step 4: Start Using Codex

Once configured, go to your code working directory and run the following command in the terminal to start using Codex.

```sh
export CODEX_CUBEROUTER_API_KEY=<your_cuberouter_api_key>
codex
```
