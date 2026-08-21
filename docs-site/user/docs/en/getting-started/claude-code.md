# Claude Code

> How to use CubeRouter in Claude Code

Claude Code is an intelligent coding tool that runs in the terminal. It interacts through natural-language commands and helps developers quickly complete code generation, debugging, refactoring, and more.

## Step 1: Install Claude Code

Prerequisites:

- You need [Node.js 18 or newer](https://nodejs.org/en/download/)
- On macOS, we recommend installing Node.js via [nvm](https://github.com/nvm-sh/nvm) or [Homebrew](https://formulae.brew.sh/formula/node). Installing from a package installer is not recommended (you may run into permission issues later)
- Windows users also need [Git for Windows](https://git-scm.com/install/windows)

Open a terminal and install Claude Code:

```bash
npm install -g @anthropic-ai/claude-code
```

Run the following command to verify the installation — a version number means it installed successfully:

```bash
claude --version
```

## Step 2: Get Your CubeRouter API Key and Model ID

1. **Register an account**: visit the CubeRouter platform, register an account and sign in (see [Register & Sign In](./register-login.md))
2. **Get an API key**: create a new API key on the **API Keys** page (see [Quick Start](./quick-start.md#step-2-create-an-api-key))
3. **Get a model ID**: on the **Model Square** page, use the **Group** filter to find the models available to your key's group, and copy the model ID for configuration

## Step 3: Configure Claude Code

::: warning Important
macOS & Linux & Windows are all supported — note that the config file path differs between systems. Also make sure the JSON file you edit stays valid (e.g. don't add or drop a stray `,`).
:::

Set the environment variables by editing the config file on **macOS**, **Linux**, or **Windows**:

```jsonc
// Edit or create the settings.json file
// macOS & Linux: ~/.claude/settings.json
// Windows: user-profile-dir/.claude/settings.json
// Add or modify the env field inside
// Replace your_platform_api_key with the API key you obtained
// Replace your_model_id with the model ID you obtained
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "your_platform_api_key",
    "ANTHROPIC_BASE_URL": "https://your-platform.com",
    "API_TIMEOUT_MS": "3000000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": 1,
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "your_model_id",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "your_model_id",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "your_model_id"
  }
}
```

Then edit or create the `.claude.json` file (macOS & Linux: `~/.claude.json`, Windows: `user-profile-dir/.claude.json`) and add the `hasCompletedOnboarding` field:

```json
{
  "hasCompletedOnboarding": true
}
```

## Step 4: Start Using Claude Code

Once configured, go to your code working directory and run the `claude` command in the terminal to start using Claude Code.

> If you see a "Do you want to use this API key" prompt, just choose Yes.

After startup, allow Claude Code to trust access to the files in the folder, and you can start developing.
