# Claude Shell

Claude Shell or GPT-Term is a terminal-based chat interface for interacting with Claude AI, specifically designed for command-line operations and bash assistance. It provides a clean, intuitive TUI (Terminal User Interface) with features like conversation history, message editing, and direct command execution.

A video is worth more than a 1000 words:

https://github.com/user-attachments/assets/645b6296-0969-47ee-938a-e88e9d207c96

## Features

- 🤖 Interactive chat with Claude AI optimized for bash/terminal assistance
- 💻 Direct command execution from AI responses
- 📝 Message editing and history navigation
- 📋 Copy messages to clipboard or select text to copy with mouse
- 🔍 Full conversation history browsing
- 🎨 Beautiful TUI with color-coded messages and syntax highlighting
- 🖱️ Mouse support for scrolling
- ⌨️ Vim-style navigation in edit mode

## Getting Started
```bash
brew tap nicolasdeory/gpt-term
brew install gpt-term
CLAUDE_API_KEY=your-api-key-here
```

## Development Installation

1. Ensure you have Go 1.22 or later installed
2. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/gpt-term.git
   cd gpt-term
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Build the project:
   ```bash
   go build ./cmd/gpt-term
   ```

## Configuration

1. Set your Claude API key as an environment variable:
   ```bash
   export CLAUDE_API_KEY='your-api-key-here'
   ```
2. (Optional) Add it to your shell's rc file (e.g., `.bashrc` or `.zshrc`) to make it permanent.

## Usage

### Basic Operation

Start the application:
```bash
./gpt-term
```

### Keyboard Shortcuts

- **Navigation & Modes**
  - `Ctrl+J/K`: Enter edit mode and navigate through messages with J/K
  - `Ctrl+N`: Create new chat
  - `Ctrl+R`: Browse conversation history
  - `Ctrl+H`: Show help
  - `Ctrl+C`: Quit
  - `ESC`: Exit current mode

- **Message Interaction**
  - `Enter`: Edit selected user message (in edit mode). Will try to open your editor or nvim. After submitting, it will reset the whole conversation history and start over from that point.
  - `X`: Execute command from selected assistant message
  - `Ctrl+X`: Execute command from last assistant message
  - `C`: Copy selected message to clipboard (in edit mode)

- **Scrolling**
  - `↑/↓`: Scroll up/down
  - `PgUp/PgDn`: Scroll by page
  - `Home/End`: Jump to top/bottom
  - Mouse wheel: Scroll up/down

### Command Execution

When an AI response contains commands (highlighted in green), you can:
1. Press `X` in edit mode to execute the command
2. If multiple commands are present, use numbers or arrow keys to select which one to execute

### Message Editing

1. Enter edit mode with `Ctrl+J` or `Ctrl+K`
2. Navigate to the message you want to edit
3. Press `Enter` to open your default editor ($EDITOR)
4. Save and exit the editor to update the message

## Storage

Conversations are automatically saved in `~/.gpt-term/conversations/` and can be browsed using `Ctrl+R`.

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Claude API](https://anthropic.com/claude) - AI model backend

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

- Built with [Charm](https://charm.sh/) libraries
- Powered by Claude AI from Anthropic
