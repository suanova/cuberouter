# OpenClaw

> How to use the CubeRouter service in OpenClaw

OpenClaw is a personal AI assistant that runs on your own device and can connect to various messaging platforms. It can use a wide range of models through the CubeRouter service.

## Step 1: Install OpenClaw

> For the full installation guide, see the [official documentation](https://docs.openclaw.ai/zh-CN/install)

### Option 1: Install script (recommended)

Prerequisites:

- [Node.js 22 or newer](https://nodejs.org/en/download/)

The simplest way to install OpenClaw is the official install script:

::: code-group

```bash [macOS/Linux]
curl -fsSL https://openclaw.ai/install.sh | bash
```

```powershell [Windows (PowerShell)]
iwr -useb https://openclaw.ai/install.ps1 | iex
```

:::

### Option 2: Global install (manual)

Prerequisites:

- [Node.js 22 or newer](https://nodejs.org/en/download/)

::: code-group

```bash [pnpm (recommended)]
pnpm add -g openclaw@latest
pnpm approve-builds -g # approve builds for openclaw, node-llama-cpp, sharp, etc.
```

```bash [npm]
npm install -g openclaw@latest
```

:::

## Step 2: Get Your CubeRouter API Key and Model ID

1. **Register an account**: visit the CubeRouter platform, register an account and sign in (see [Register & Sign In](./register-login.md))
2. **Get an API key**: create a new API key on the **API Keys** page (see [Quick Start](./quick-start.md#step-2-create-an-api-key))
3. **Get a model ID**: on the **Model Square** page, use the **Group** filter to find the models available to your key's group, and copy the model ID for configuration

## Step 3: Set Up OpenClaw

After running the install command above, the setup wizard starts automatically. If it doesn't, you can start it with:

```bash
openclaw onboard --install-daemon
```

If you already initialized before, you can also run `openclaw config` and select `model`.

Start the setup:

- `I understand this is powerful and inherently risky. Continue?`: select `Yes`
- `Onboarding mode`: select `Quick Start`
- `Model/auth provider`: select `Custom Provider`

Configure the CubeRouter provider:

- `API Base URL`: enter `https://your-platform.com/v1`
- `API Key`: enter the API key you created
- `Endpoint compatibility`: select `OpenAI-compatible`
- `Model ID`: enter the model ID

#### Finish the setup

Continue configuring the remaining OpenClaw features:

- `Select channel`: select and configure the features you need
- `Configure skills`: select and install the skills you need
- Complete the setup

#### Interact with your bot

After setup, the CLI asks `How do you want to hatch your bot?`

- Select `Hatch in TUI (recommended)`

Now you can start chatting with your bot in the Terminal UI.

OpenClaw offers more channels to interact with your bot, such as Web UI, Discord, Slack, and more. See the official documentation to set those up: [Channels Setup](https://docs.openclaw.ai/channels)

- For the Web UI, you can open the `Web UI (with token)` link shown in the terminal to access it

#### Verify after install

Verify that everything works:

```bash
openclaw doctor    # check for configuration problems
openclaw status    # view gateway status
openclaw dashboard # open the Dashboard in the browser
```

> For the full configuration guide, see the [official documentation](https://docs.openclaw.ai/zh-CN/start/getting-started)

::: warning Important
If misconfigured or deployed without proper access control, OpenClaw can pose security risks. See the [official security documentation](https://docs.openclaw.ai/gateway/security)
:::

## Advanced Configuration

### Model failover

Configure model failover for reliability (`.openclaw/openclaw.json`):

```json
{
  "agents": {
    "defaults": {
      "model": {
        "primary": "custom-provider/model-id-a",
        "fallbacks": ["custom-provider/model-id-b", "custom-provider/model-id-c"]
      }
    }
  }
}
```

### Using skills

> A skill is a folder containing a SKILL.md file. If you want to add new capabilities to your OpenClaw agent, [ClawHub](https://clawhub.ai/) is the best place to find and install skills.

Install clawhub:

```bash
npm i -g clawhub
```

Manage skills:

```bash
clawhub search "postgres backups"  # search skills
clawhub install my-skill-pack      # download a new skill
clawhub update --all               # update installed skills
```

### Plugin management

> A plugin is a small code module that extends OpenClaw with extra features (commands, tools, and Gateway RPC).

```bash
openclaw plugins list                      # list loaded plugins
openclaw plugins install @openclaw/voice-call  # install an official plugin (example: voice-call)
openclaw gateway restart                   # restart the gateway
```

## Troubleshooting

### Common issues

1. **API key authentication**
   - Make sure your API key is valid and hasn't hit its quota limit
   - Check that the API key is set correctly in the environment

2. **Model availability**
   - Verify the model is available in your group
   - Check that the model name (model ID) format is correct

3. **Connection problems**
   - Make sure the OpenClaw gateway is running
   - Check network connectivity to the CubeRouter endpoint

## Resources

- **OpenClaw docs**: [docs.openclaw.ai](https://docs.openclaw.ai/)
- **OpenClaw GitHub**: [github.com/openclaw/openclaw](https://github.com/openclaw/openclaw)
- **Community skills**: [awesome-openclaw-skills](https://github.com/VoltAgent/awesome-openclaw-skills)
