# Playground

> The Playground is CubeRouter's online testing and chat tool for AI models. Chat directly with models without writing any code — perfect for quickly verifying keys and model behavior.

Click **Playground** in the left navigation, or go directly to `/playground`.

![Playground page](/imgs-en/playground.jpeg)

## Feature Overview

- 🎛️ **Model configuration**: flexibly configure model parameters for different scenarios
- 💬 **Real-time chat**: test real-time conversations with AI models
- 📊 **Debug mode**: view full request and response details
- 💾 **Configuration management**: save, import, and export model configurations

## Using the Playground

1. Select the model you want to test at the top of the page
2. Type your message in the input box at the bottom and click Send
3. The model's reply appears in the conversation area

![Playground chat example](/imgs-en/playground.jpeg)

Below the reply, the response time is displayed along with the following action buttons:

- **Copy**: copy the message content with one click
- **Show source**: view the raw Markdown content
- **Regenerate**: regenerate the current reply
- **Edit**: edit a sent message
- **Delete**: delete the current message

The page also provides **Getting-started prompts** shortcut buttons (Analyze data, Summarize text, Code, Get suggestions) to help you start testing quickly.

## Parameter Settings

Click the **Parameters** button next to the input box to open the parameter settings dialog and configure model parameters. Only enabled parameters are sent with the request.

![Parameter settings](/imgs-en/playground-params.jpeg)

### Temperature

- Controls the randomness and creativity of the output
- Range: 0.0 - 2.0, default: 0.7
- Low temperature (0.0 - 0.3): more deterministic, consistent output; suitable for code generation
- High temperature (0.8 - 2.0): more random, creative output; suitable for creative writing

### Top P

- Controls the diversity of word selection
- Range: 0.0 - 1.0, default: 1.0
- Usually no adjustment needed; adjust Temperature first

### Frequency Penalty

- Reduces the frequency of repeated words
- Range: -2.0 - 2.0, default: 0
- Positive values: penalize words that have already appeared, avoiding repetition

### Presence Penalty

- Encourages discussing new topics
- Range: -2.0 - 2.0, default: 0
- Positive values: encourage discussing new topics

### Max Tokens

- Controls the maximum number of output tokens
- Default: 4096
- Mind the model's maximum token limit

### Seed

- Used to reproduce the same output
- Leave empty and the output may differ each time

## Parameter Recommendations

**Code generation**:

- Temperature: 0.2
- Top P: 0.5
- Frequency Penalty: 0.0
- Presence Penalty: 0.0

**Creative writing**:

- Temperature: 0.9
- Top P: 0.9
- Frequency Penalty: 0.3
- Presence Penalty: 0.6

**Q&A chat**:

- Temperature: 0.7
- Top P: 1.0
- Frequency Penalty: 0.0
- Presence Penalty: 0.0

## Use Cases

### Quick-test a model

1. Open the Playground
2. Select the model you want to test
3. Enter a question
4. View the model's reply
5. Adjust the parameters and test again

### Tuning generation quality

1. Enter a question and view the initial reply
2. If you're not satisfied, adjust the parameters:
   - Increase Temperature for more creativity
   - Lower Temperature for more precision
   - Adjust Top P to control diversity
3. Regenerate the reply

## Core Value

1. **Quick testing**: test model behavior without writing code
2. **Parameter tuning**: intuitively adjust parameters to find the best configuration
3. **Real-time feedback**: see the impact of parameter changes instantly
4. **Configuration management**: save, share, and import configurations
