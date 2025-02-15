package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/google/uuid"

	"flag"
	"gpt-term/internal/claude"
	"gpt-term/internal/storage"
)

// New message types for asynchronous commands

type apiResponseMsg struct {
	response string
	err      error
}

type editMessageMsg struct {
	index  int
	edited string
	err    error
}

// Add new message type for command output
type commandOutputMsg struct {
	output string
	err    error
}

// Add new message type for scrolling
type scrollMsg struct {
	offset int
}

// model now includes spinner and loading flag

type model struct {
	textInput       textinput.Model
	viewport        viewport.Model
	err             error
	conversation    *storage.Conversation
	mode            Mode
	messages        []storage.Message
	cursorIndex     int
	storage         *storage.Storage
	client          *claude.Client
	conversations   []storage.Conversation
	selectedConv    int
	spinner         spinner.Model
	isLoading       bool
	height          int
	width           int
	commands        [][]string
	selectedCommand int
	ready           bool // Add this field to track if window size is set
	lastLoadedConv  int  // Add this new field
}

type Mode int

const (
	ModeNormal Mode = iota
	ModeEditing
	ModeHistory
	ModeCommandSelect
	ModeHelp
)

var (
	focusedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	botStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("82")).Foreground(lipgloss.Color("0"))
	userStyle     = lipgloss.NewStyle().Background(lipgloss.Color("255")).Foreground(lipgloss.Color("0"))
	systemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	commandStyle  = lipgloss.NewStyle().
			Background(lipgloss.Color("82")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1)
	titleStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("82")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			MarginBottom(1)
	scrollIndicatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	userLabelStyle       = lipgloss.NewStyle().
				Background(lipgloss.Color("33")).  // Blue bg
				Foreground(lipgloss.Color("255")). // White text
				Padding(0, 1)                      // Add some padding
	assistantLabelStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("208")). // Orange bg
				Foreground(lipgloss.Color("0")).   // Black text
				Padding(0, 1)                      // Add some padding
	messageStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("242")) // Gray text for user messages
	codeBlockStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")). // Dark gray background
			Padding(0, 2).                     // Add horizontal padding
			MarginLeft(2)                      // Indent the block
	selectedLabelStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("226")). // Yellow bg
				Foreground(lipgloss.Color("0")).   // Black text
				Padding(0, 1)                      // Add some padding
	instructionBarStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("226")). // Yellow bg
				Foreground(lipgloss.Color("0")).   // Black text
				Width(80).                         // Fixed width for the bar
				MarginLeft(2)                      // Match the left margin
	overlayStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("0")).       // Black background
			Padding(1, 2).                         // Add some padding
			Border(lipgloss.RoundedBorder()).      // Add a border
			BorderForeground(lipgloss.Color("82")) // Green border
	selectedMessageStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("226")). // Yellow bg
				PaddingLeft(1).                    // Small padding
				PaddingRight(1)                    // Small padding
)

const (
	upArrow   = "▲"
	downArrow = "▼"
	endText   = ""
	version   = "1.0.0"
)

const systemPrompt = `You are a bash terminal helper AI. Unless the user asks otherwise, you will specify all solutions in bash commands ideally one liners if its simple. Before displaying the bash command code, you must surround it with <command></command> tags. Each <command> block must contain exactly one command - if you need to show multiple commands, use multiple <command> blocks. Do not insert `

const helpMessage = `GPT Terminal Help:
- Ctrl+J/K: Enter edit mode and navigate through messages
- Enter: Edit selected user message
- X: Execute command from selected assistant message
- Alt+X: Execute command from last assistant message
- Ctrl+R: Browse conversation history
- Ctrl+L: Load latest conversation
- Ctrl+N: Create new chat
- Ctrl+C: Quit
- Ctrl+H: Show this help

Commands in responses are highlighted and can be executed. If multiple commands are present, you'll be prompted to choose one.`

func initialModel() (model, error) {
	ti := textinput.New()
	ti.Placeholder = "What do you want to ask?"
	ti.Focus()
	ti.CharLimit = 156

	store, err := storage.NewStorage()
	if err != nil {
		return model{}, fmt.Errorf("error creating storage: %w", err)
	}

	conv := &storage.Conversation{
		ID:        uuid.New().String(),
		CreatedAt: time.Now(),
		Messages:  make([]storage.Message, 0),
	}

	sp := spinner.NewModel()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	sp.Spinner = spinner.Points

	// Initialize viewport with default dimensions
	vp := viewport.New(0, 0) // We'll set actual dimensions when we get WindowSizeMsg
	vp.Style = lipgloss.NewStyle().Margin(1, 2)
	vp.KeyMap = viewport.KeyMap{} // Clear default keybindings to avoid conflicts

	// Add system prompt as hidden message
	systemMsg := storage.Message{
		Role:      "system",
		Content:   systemPrompt,
		Timestamp: time.Now(),
	}
	conv.Messages = append(conv.Messages, systemMsg)

	return model{
		textInput:      ti,
		viewport:       vp,
		mode:           ModeNormal,
		conversation:   conv,
		messages:       conv.Messages,
		storage:        store,
		client:         claude.NewClient(),
		spinner:        sp,
		isLoading:      false,
		ready:          false,
		lastLoadedConv: -1, // Initialize to -1
	}, nil
}

func (m model) Init() tea.Cmd {
	// Get initial terminal size
	width, height, err := term.GetSize(uintptr(os.Stdout.Fd()))
	if err == nil && width != 0 && height != 0 {
		m.width = width
		m.height = height
		m.ready = true
		m.updateViewport()
	}
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Always update spinner if loading
	if m.isLoading {
		var sCmd tea.Cmd
		m.spinner, sCmd = m.spinner.Update(msg)
		cmds = append(cmds, sCmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.ready = true
		// Update text input width to use full width (minus margins)
		m.textInput.Width = m.width - 4 // Account for left and right margins
		m.updateViewport()
		return m, nil

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			if m.mode == ModeHistory {
				oldSelected := m.selectedConv
				m.selectedConv = max(0, m.selectedConv-1)
				if oldSelected != m.selectedConv {
					m.ensureConversationVisible(m.selectedConv)
				}
				return m, nil
			} else if m.mode == ModeEditing {
				m.viewport.LineUp(3)
			} else {
				m.viewport.LineUp(3)
			}
			return m, nil
		case tea.MouseWheelDown:
			if m.mode == ModeHistory {
				oldSelected := m.selectedConv
				m.selectedConv = min(len(m.conversations)-1, m.selectedConv+1)
				if oldSelected != m.selectedConv {
					m.ensureConversationVisible(m.selectedConv)
				}
				return m, nil
			} else if m.mode == ModeEditing {
				m.viewport.LineDown(3)
			} else {
				m.viewport.LineDown(3)
			}
			return m, nil
		}

	case tea.KeyMsg:
		// First handle mode-independent keys
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+x":
			return m.handleCommandExecution()
		case "ctrl+j", "ctrl+k":
			m.mode = ModeEditing
			m.cursorIndex = len(m.messages) - 1
			m.updateViewport()
			return m, nil
		case "ctrl+l":
			// Load conversations
			conversations, err := m.storage.ListConversations()
			if err != nil {
				m.err = err
				return m, nil
			}

			if len(conversations) > 0 {
				// Sort conversations by date
				sort.Slice(conversations, func(i, j int) bool {
					return conversations[i].CreatedAt.After(conversations[j].CreatedAt)
				})

				// Increment lastLoadedConv or wrap around to 0
				m.lastLoadedConv++
				if m.lastLoadedConv >= len(conversations) {
					m.lastLoadedConv = 0
				}

				// Load the next conversation
				m.conversation = &conversations[m.lastLoadedConv]
				m.messages = m.conversation.Messages
				m.updateViewport()
				m.viewport.GotoBottom()
			}
			return m, nil
		case "ctrl+n":
			// Create new conversation
			conv := &storage.Conversation{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				Messages:  make([]storage.Message, 0),
			}
			// Add system prompt as hidden message
			systemMsg := storage.Message{
				Role:      "system",
				Content:   systemPrompt,
				Timestamp: time.Now(),
			}
			conv.Messages = append(conv.Messages, systemMsg)

			// Update model with new conversation
			m.conversation = conv
			m.messages = conv.Messages
			m.mode = ModeNormal
			m.updateViewport()
			return m, nil
		case "ctrl+h":
			m.mode = ModeHelp
			m.updateViewport()
			return m, nil
		}

		// Then handle mode-specific keys
		switch m.mode {
		case ModeNormal:
			// Handle viewport scrolling keys first
			switch msg.String() {
			case "up":
				m.viewport.LineUp(3)
				return m, nil // Return immediately to prevent updateViewport
			case "down":
				m.viewport.LineDown(3)
				return m, nil // Return immediately to prevent updateViewport
			case "pgup":
				m.viewport.HalfViewUp()
				return m, nil // Return immediately to prevent updateViewport
			case "pgdn":
				m.viewport.HalfViewDown()
				return m, nil // Return immediately to prevent updateViewport
			case "home":
				m.viewport.GotoTop()
				return m, nil // Return immediately to prevent updateViewport
			case "end":
				m.viewport.GotoBottom()
				return m, nil // Return immediately to prevent updateViewport
			}

			// Then handle normal mode specific keys
			switch msg.Type {
			case tea.KeyEsc:
				return m, tea.Quit
			case tea.KeyEnter:
				if m.textInput.Value() != "" {
					userMsg := storage.Message{
						Role:      "user",
						Content:   m.textInput.Value(),
						Timestamp: time.Now(),
					}
					m.messages = append(m.messages, userMsg)
					m.conversation.Messages = m.messages
					m.updateViewport()
					m.viewport.GotoBottom()

					var claudeMsgs []claude.Message
					for _, msg := range m.messages {
						claudeMsgs = append(claudeMsgs, claude.Message{
							Role:    msg.Role,
							Content: msg.Content,
						})
					}

					m.isLoading = true
					m.textInput.Reset()
					return m, func() tea.Msg {
						response, err := m.client.CreateMessage(claudeMsgs)
						return apiResponseMsg{response: response, err: err}
					}
				}
			case tea.KeyRunes:
				if msg.Alt {
					switch msg.String() {
					case "j", "k":
						m.mode = ModeEditing
						m.cursorIndex = len(m.messages) - 1
						m.updateViewport()
						return m, nil
					}
				}
			case tea.KeyCtrlR:
				m.mode = ModeHistory
				conversations, err := m.storage.ListConversations()
				if err != nil {
					m.err = err
					return m, nil
				}
				m.conversations = conversations
				m.selectedConv = 0
				m.updateViewport()
			case tea.KeyCtrlH:
				m.mode = ModeHelp
				return m, nil
			}

			// Finally update text input
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)

		case ModeEditing:
			switch msg.Type {
			case tea.KeyEsc:
				m.mode = ModeNormal
				m.updateViewport()
			case tea.KeyRunes:
				switch msg.String() {
				case "k":
					if m.cursorIndex > 1 { // Start from 1 to skip system prompt
						m.cursorIndex--
						m.ensureMessageVisible(m.cursorIndex)
						return m, nil // Return immediately to prevent updateViewport
					}
				case "j":
					if m.cursorIndex < len(m.messages)-1 {
						m.cursorIndex++
						m.ensureMessageVisible(m.cursorIndex)
						return m, nil // Return immediately to prevent updateViewport
					}
				case "x":
					if m.messages[m.cursorIndex].Role == "assistant" {
						return m.handleCommandExecution()
					}
				case "c":
					// Copy current message to clipboard
					if m.cursorIndex < len(m.messages) {
						msg := m.messages[m.cursorIndex]
						cmd, err := getClipboardCommand()
						if err != nil {
							m.err = err
							return m, nil
						}
						cmd.Stdin = strings.NewReader(msg.Content)
						m.mode = ModeNormal // Set mode back to normal before executing command
						return m, tea.ExecProcess(
							cmd,
							func(err error) tea.Msg {
								if err != nil {
									return nil
								}
								return nil
							},
						)
					}
				}
			case tea.KeyUp:
				m.viewport.LineUp(3)
				return m, nil
			case tea.KeyDown:
				m.viewport.LineDown(3)
				return m, nil
			case tea.KeyEnter:
				if m.messages[m.cursorIndex].Role == "user" {
					return m, editMessageCmd(m.messages[m.cursorIndex].Content, m.cursorIndex)
				}
				m.mode = ModeNormal
				m.updateViewport()
			}

		case ModeHistory:
			switch msg.Type {
			case tea.KeyEsc:
				m.mode = ModeNormal
				m.updateViewport()
			case tea.KeyUp:
				oldSelected := m.selectedConv
				m.selectedConv = max(0, m.selectedConv-1)
				if oldSelected != m.selectedConv {
					m.ensureConversationVisible(m.selectedConv)
				}
				return m, nil
			case tea.KeyDown:
				oldSelected := m.selectedConv
				m.selectedConv = min(len(m.conversations)-1, m.selectedConv+1)
				if oldSelected != m.selectedConv {
					m.ensureConversationVisible(m.selectedConv)
				}
				return m, nil
			case tea.KeyPgUp:
				oldSelected := m.selectedConv
				m.selectedConv = max(0, m.selectedConv-m.viewport.Height)
				if oldSelected != m.selectedConv {
					m.ensureConversationVisible(m.selectedConv)
				}
				return m, nil
			case tea.KeyPgDown:
				oldSelected := m.selectedConv
				m.selectedConv = min(len(m.conversations)-1, m.selectedConv+m.viewport.Height)
				if oldSelected != m.selectedConv {
					m.ensureConversationVisible(m.selectedConv)
				}
				return m, nil
			case tea.KeyHome:
				m.selectedConv = 0
				m.ensureConversationVisible(m.selectedConv)
				return m, nil
			case tea.KeyEnd:
				m.selectedConv = len(m.conversations) - 1
				m.ensureConversationVisible(m.selectedConv)
				return m, nil
			case tea.KeyEnter:
				if len(m.conversations) > 0 {
					// Create sorted copy of conversations
					sortedConvs := make([]storage.Conversation, len(m.conversations))
					copy(sortedConvs, m.conversations)
					sort.Slice(sortedConvs, func(i, j int) bool {
						return sortedConvs[i].CreatedAt.After(sortedConvs[j].CreatedAt)
					})

					// Use the sorted conversations for selection
					m.conversation = &sortedConvs[m.selectedConv]
					m.messages = m.conversation.Messages
					m.mode = ModeNormal
					m.updateViewport()
					m.viewport.GotoBottom()
				}
			}

		case ModeCommandSelect:
			switch msg.Type {
			case tea.KeyEsc:
				m.mode = ModeNormal
			case tea.KeyUp:
				if m.selectedCommand > 0 {
					m.selectedCommand--
				}
			case tea.KeyDown:
				if m.selectedCommand < len(m.commands)-1 {
					m.selectedCommand++
				}
			case tea.KeyEnter:
				if len(m.commands) > 0 {
					cmdStr := m.commands[m.selectedCommand][1]
					m.mode = ModeNormal
					return m, executeCommand(cmdStr)
				}
			case tea.KeyRunes:
				switch msg.String() {
				case "c":
					if len(m.commands) > 0 {
						cmdStr := m.commands[m.selectedCommand][1]
						cmd, err := getClipboardCommand()
						if err != nil {
							m.err = err
							return m, nil
						}
						cmd.Stdin = strings.NewReader(cmdStr)
						m.mode = ModeNormal
						return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
							if err != nil {
								return nil
							}
							return nil
						})
					}
				default:
					// Handle numeric selection
					if num, err := strconv.Atoi(msg.String()); err == nil && num > 0 && num <= len(m.commands) {
						cmdStr := m.commands[num-1][1]
						m.mode = ModeNormal
						return m, executeCommand(cmdStr)
					}
				}
			}

		case ModeHelp:
			m.mode = ModeNormal
			m.updateViewport()
			return m, nil
		}

	case apiResponseMsg:
		m.isLoading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		botMsg := storage.Message{
			Role:      "assistant",
			Content:   msg.response,
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, botMsg)
		m.conversation.Messages = m.messages

		// Generate summary from first user message if not already set
		if m.conversation.Summary == "" {
			for _, msg := range m.messages {
				if msg.Role == "user" {
					summary := msg.Content
					if len(summary) > 50 {
						summary = summary[:47] + "..."
					}
					m.conversation.Summary = summary
					break
				}
			}
		}

		if err := m.storage.SaveConversation(m.conversation); err != nil {
			m.err = err
		}

		// Update viewport with new content
		m.updateViewport()
		m.viewport.GotoBottom()

	case editMessageMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.messages[msg.index].Content = msg.edited
		m.messages = m.messages[:msg.index+1]
		m.conversation.Messages = m.messages
		m.updateViewport()
		m.viewport.GotoBottom()

		// Regenerate summary if first user message was edited
		for _, msg := range m.messages {
			if msg.Role == "user" {
				summary := msg.Content
				if len(summary) > 50 {
					summary = summary[:47] + "..."
				}
				m.conversation.Summary = summary
				break
			}
		}

		if err := m.storage.SaveConversation(m.conversation); err != nil {
			m.err = err
		}
		m.mode = ModeNormal

		// Convert messages to Claude format and send request
		var claudeMsgs []claude.Message
		for _, msg := range m.messages {
			claudeMsgs = append(claudeMsgs, claude.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		m.isLoading = true
		return m, func() tea.Msg {
			response, err := m.client.CreateMessage(claudeMsgs)
			return apiResponseMsg{response: response, err: err}
		}

	case commandOutputMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Add command output as assistant message
		botMsg := storage.Message{
			Role:      "assistant",
			Content:   "```\n" + msg.output + "```",
			Timestamp: time.Now(),
		}
		m.messages = append(m.messages, botMsg)
		m.conversation.Messages = m.messages
		if err := m.storage.SaveConversation(m.conversation); err != nil {
			m.err = err
		}

		// Update viewport with new content and scroll to bottom
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil

	case scrollMsg:
		m.viewport.YOffset = msg.offset
		fmt.Fprintf(os.Stderr, "DEBUG: Applied scroll offset: %d\n", msg.offset)
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// editMessageCmd launches the user's preferred editor ($EDITOR) to edit the message content
func editMessageCmd(content string, index int) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim" // fallback to nvim
	}

	tmpFile, err := os.CreateTemp("", "gpt-term-edit-*.txt")
	if err != nil {
		return func() tea.Msg {
			return editMessageMsg{index: index, err: err}
		}
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		return func() tea.Msg {
			return editMessageMsg{index: index, err: err}
		}
	}
	tmpFile.Close()

	c := exec.Command(editor, tmpFile.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		defer os.Remove(tmpFile.Name())

		if err != nil {
			return editMessageMsg{index: index, err: err}
		}

		data, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			return editMessageMsg{index: index, err: err}
		}

		return editMessageMsg{index: index, edited: string(data)}
	})
}

func (m model) handleCommandExecution() (tea.Model, tea.Cmd) {
	var targetMsg string
	if m.mode == ModeEditing {
		if m.messages[m.cursorIndex].Role == "assistant" {
			targetMsg = m.messages[m.cursorIndex].Content
		}
	} else {
		// Find last assistant message
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == "assistant" {
				targetMsg = m.messages[i].Content
				break
			}
		}
	}

	if targetMsg == "" {
		return m, nil
	}

	// Use the same regex pattern as formatContent
	re := regexp.MustCompile(`(?s)<command>(.*?)</command>`)
	matches := re.FindAllStringSubmatch(targetMsg, -1)

	if len(matches) == 0 {
		return m, nil
	}

	// Clean up commands before execution
	for i := range matches {
		matches[i][1] = strings.TrimSpace(matches[i][1])
	}

	// Always show command selection, even for single commands
	m.mode = ModeCommandSelect
	m.commands = matches
	m.selectedCommand = 0

	return m, nil
}

// Add this function to handle command execution and output
func executeCommand(cmdStr string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", cmdStr)
		output, err := cmd.CombinedOutput()
		var status string
		if err != nil {
			status = fmt.Sprintf("Command failed: %v\n", err)
		} else {
			status = "Command executed successfully\n"
		}
		return commandOutputMsg{
			output: fmt.Sprintf("Command ran: %s\nCommand result:\n%s%s", cmdStr, status, string(output)),
			err:    err,
		}
	}
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Build the final view
	var finalView strings.Builder

	// Add conversation title
	if m.conversation != nil && m.conversation.Summary != "" {
		finalView.WriteString(titleStyle.Render(m.conversation.Summary))
		finalView.WriteString("\n")
	}

	// Add main content
	finalView.WriteString("  ") // Two spaces for left margin alignment
	if m.viewport.YOffset > 0 {
		finalView.WriteString(scrollIndicatorStyle.Render(upArrow))
	} else if len(m.messages) > 1 { // Only show beginning text if there are messages beyond system prompt
		finalView.WriteString(scrollIndicatorStyle.Render(endText))
	} else {
		finalView.WriteString("\n")
	}
	finalView.WriteString("\n")

	// Add main content
	finalView.WriteString(m.viewport.View())

	// Add scroll down indicator
	finalView.WriteString("\n")
	finalView.WriteString("  ") // Two spaces for left margin alignment
	if m.viewport.YOffset < m.viewport.TotalLineCount()-m.viewport.Height {
		finalView.WriteString(scrollIndicatorStyle.Render(downArrow))
	} else {
		finalView.WriteString(scrollIndicatorStyle.Render(endText))
	}

	finalView.WriteString("\n\n") // Added extra newline for margin
	finalView.WriteString(m.statusBarView())

	// If in command select mode, overlay the command selection
	if m.mode == ModeCommandSelect {
		var overlay strings.Builder
		overlay.WriteString("Select a command to execute or copy:\n\n")

		for i, match := range m.commands {
			cmd := match[1]
			line := fmt.Sprintf("%d: %s", i+1, cmd)
			if i == m.selectedCommand {
				overlay.WriteString(selectedStyle.Render(line))
			} else {
				overlay.WriteString(line)
			}
			overlay.WriteString("\n")
		}

		overlayContent := overlayStyle.Render(overlay.String())

		// Calculate position to center the overlay
		overlayLines := strings.Count(overlayContent, "\n") + 1
		viewportMiddle := m.height / 2
		overlayStart := viewportMiddle - overlayLines/2

		// Split the final view into lines
		lines := strings.Split(finalView.String(), "\n")

		// Insert the overlay in the middle
		var result strings.Builder
		for i := 0; i < len(lines); i++ {
			if i == overlayStart {
				result.WriteString(overlayContent)
				result.WriteString("\n")
			}
			if i < len(lines) {
				result.WriteString(lines[i])
				if i < len(lines)-1 {
					result.WriteString("\n")
				}
			}
		}

		return result.String()
	}

	return finalView.String()
}

// Helper function for debug info
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) statusBarView() string {
	var status string
	if m.isLoading {
		status = m.spinner.View() + " Loading..."
	}
	switch m.mode {
	case ModeNormal:
		return fmt.Sprintf("%s\n%s\n↑/↓: Scroll | Ctrl+J/K: Edit | Ctrl+X/X: Execute | Ctrl+R: History | Ctrl+N: New chat | Ctrl+H: Show full help",
			m.textInput.View(), status)
	case ModeEditing:
		return "Press ESC to exit, J/K to navigate messages, Enter to edit message, X to execute command, C to copy message"
	case ModeHistory:
		return "Press ESC to exit, Enter to select conversation, Up/Down/MWheel to scroll"
	case ModeCommandSelect:
		if len(m.commands) == 1 {
			return "Press Enter to execute command, C to copy command, ESC to cancel"
		}
		return "Press ESC to exit, Enter/number to execute selected command, C to copy selected command"
	case ModeHelp:
		return "Press any key to exit help"
	default:
		return ""
	}
}

func formatContent(content string) string {
	// First handle code blocks - make regex more permissive to catch all variants
	re := regexp.MustCompile("(?s)```.*?\n(.*?)```")
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the code content without the backticks and language identifier
		code := re.FindStringSubmatch(match)[1]
		return "\n" + codeBlockStyle.Render(code) + "\n"
	})

	// Then handle commands - make sure to handle newlines properly
	cmdRe := regexp.MustCompile(`(?s)<command>(.*?)</command>`)
	content = cmdRe.ReplaceAllStringFunc(content, func(match string) string {
		cmd := cmdRe.FindStringSubmatch(match)[1]
		// Trim any whitespace/newlines around the command
		cmd = strings.TrimSpace(cmd)
		return commandStyle.Render(cmd)
	})

	return content
}

func (m model) normalView() string {
	var s strings.Builder

	for _, msg := range m.messages {
		if msg.Role == "system" {
			// Only show beginning text with timestamp for existing conversations
			// (ones that have more than just the system message)
			if len(m.messages) > 1 {
				beginningText := fmt.Sprintf("- Beginning of conversation [%s] -",
					m.conversation.CreatedAt.Format("Mon 02 Jan 2006 15:04"))
				s.WriteString(scrollIndicatorStyle.Render(beginningText) + "\n\n")
			}
			continue
		}
		switch msg.Role {
		case "assistant":
			content := formatContent(msg.Content)
			s.WriteString(assistantLabelStyle.Render("assistant") + " " + botStyle.Render(content) + "\n\n")
		default:
			s.WriteString(userLabelStyle.Render("user") + " " + messageStyle.Render(msg.Content) + "\n\n")
		}
	}

	return s.String()
}

func (m model) editingView() string {
	var s strings.Builder
	s.WriteString("Editing Mode\n\n")

	for i, msg := range m.messages {
		var content string
		if msg.Role == "assistant" {
			content = formatContent(msg.Content)
		}

		if i == m.cursorIndex {
			switch msg.Role {
			case "system":
				s.WriteString(systemStyle.Render(fmt.Sprintf("%s: %s", msg.Role, msg.Content)))
			case "user":
				s.WriteString(selectedLabelStyle.Render("user") + " " + selectedMessageStyle.Render(msg.Content))
				s.WriteString("\n" + instructionBarStyle.Render("Press Enter to edit, C to copy message"))
			case "assistant":
				s.WriteString(selectedLabelStyle.Render("assistant") + " " + selectedMessageStyle.Render(content))
				// Show appropriate instructions based on message content
				if strings.Contains(msg.Content, "<command>") {
					s.WriteString("\n" + instructionBarStyle.Render("Press X to execute commands, C to copy message"))
				} else {
					s.WriteString("\n" + instructionBarStyle.Render("Press C to copy message"))
				}
			}
		} else {
			switch msg.Role {
			case "system":
				s.WriteString(systemStyle.Render(fmt.Sprintf("%s: %s", msg.Role, msg.Content)))
			case "user":
				s.WriteString(userLabelStyle.Render("user") + " " + messageStyle.Render(msg.Content))
			case "assistant":
				s.WriteString(assistantLabelStyle.Render("assistant") + " " + botStyle.Render(content))
			}
		}
		s.WriteString("\n\n")
	}

	return s.String()
}

func (m model) historyView() string {
	s := "Conversation History (Press ESC to exit)\n\n"

	// Sort conversations by date in descending order
	sortedConvs := make([]storage.Conversation, len(m.conversations))
	copy(sortedConvs, m.conversations)
	sort.Slice(sortedConvs, func(i, j int) bool {
		return sortedConvs[i].CreatedAt.After(sortedConvs[j].CreatedAt)
	})

	for i, conv := range sortedConvs {
		line := fmt.Sprintf("[%s] %s", conv.CreatedAt.Format("2006-01-02 15:04:05"), conv.Summary)
		if i == m.selectedConv {
			s += selectedStyle.Render(line) + "\n"
		} else {
			s += line + "\n"
		}
	}

	// Add extra newline at the end to ensure last entry is fully visible
	s += "\n"
	return s
}

func (m model) commandSelectView() string {
	var s strings.Builder

	if len(m.commands) == 1 {
		s.WriteString("Confirm command execution:\n\n")
		cmd := m.commands[0][1]
		if m.selectedCommand == 0 {
			s.WriteString(selectedStyle.Render(cmd))
		} else {
			s.WriteString(cmd)
		}
		s.WriteString("\n\nPress Enter to execute, ESC to cancel")
	} else {
		s.WriteString("Select a command to execute:\n\n")
		for i, match := range m.commands {
			cmd := match[1]
			line := fmt.Sprintf("%d: %s", i+1, cmd)
			if i == m.selectedCommand {
				s.WriteString(selectedStyle.Render(line))
			} else {
				s.WriteString(line)
			}
			s.WriteString("\n")
		}
	}

	return s.String()
}

func (m model) helpView() string {
	return helpMessage
}

func (m *model) ensureMessageVisible(index int) (tea.Model, tea.Cmd) {
	// Generate content and set it first
	content := m.editingView()
	m.viewport.SetContent(content)

	// Now find our target message position
	lines := strings.Split(content, "\n")
	var targetLine int
	currentMsg := -1
	for i, line := range lines {
		// Look for the styled labels that appear in the actual rendered content
		if strings.Contains(line, userLabelStyle.Render("user")) ||
			strings.Contains(line, assistantLabelStyle.Render("assistant")) ||
			strings.Contains(line, selectedLabelStyle.Render("user")) ||
			strings.Contains(line, selectedLabelStyle.Render("assistant")) {
			currentMsg++
			if currentMsg == index {
				targetLine = i
				break
			}
		}
	}

	// Calculate viewport constraints
	totalLines := len(lines)
	maxScroll := totalLines - m.viewport.Height
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Calculate desired position - aim for 1/4 of the viewport height above the target
	// For the last message, aim to show it at the bottom
	desiredOffset := targetLine - (m.viewport.Height / 4)
	if index == len(m.messages)-1 {
		desiredOffset = maxScroll
	}

	// Clamp to valid bounds
	if desiredOffset < 0 {
		desiredOffset = 0
	}
	if desiredOffset > maxScroll {
		desiredOffset = maxScroll
	}

	// First go to top
	m.viewport.GotoTop()

	// Then scroll down line by line to reach our target
	for i := 0; i < desiredOffset; i++ {
		m.viewport.LineDown(1)
	}

	return m, nil
}

func (m *model) ensureConversationVisible(index int) {
	// Generate content and set it first
	content := m.historyView()
	m.viewport.SetContent(content)

	// Find target conversation position
	lines := strings.Split(content, "\n")
	targetLine := index + 2 // Add 2 to account for header lines

	// Calculate viewport constraints
	totalLines := len(lines)
	maxScroll := totalLines - m.viewport.Height + 1 // Add 1 to account for footer space
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Calculate desired position - aim for middle of viewport
	desiredOffset := targetLine - (m.viewport.Height / 2)

	// Clamp to valid bounds
	if desiredOffset < 0 {
		desiredOffset = 0
	}
	if desiredOffset > maxScroll {
		desiredOffset = maxScroll
	}

	// Update viewport position
	m.viewport.YOffset = desiredOffset
}

func (m *model) updateViewport() {
	// Store current scroll position
	currentOffset := m.viewport.YOffset

	// Update viewport dimensions
	m.viewport.Width = m.width - 4
	m.viewport.Height = m.height - 7

	// Generate content based on current mode
	var content string
	switch m.mode {
	case ModeNormal:
		content = m.normalView()
	case ModeEditing:
		content = m.editingView()
	case ModeHistory:
		content = m.historyView()
	case ModeCommandSelect:
		content = m.commandSelectView()
	case ModeHelp:
		content = helpMessage
	default:
		content = "Unknown mode"
	}

	// Set content
	m.viewport.SetContent(content)

	// For help mode, always scroll to top
	if m.mode == ModeHelp {
		m.viewport.GotoTop()
		return
	}

	// Calculate maximum valid scroll position
	maxOffset := m.viewport.TotalLineCount() - m.viewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}

	// Try to restore previous scroll position, clamped to valid range
	if currentOffset >= 0 && currentOffset <= maxOffset {
		m.viewport.YOffset = currentOffset
	} else if currentOffset > maxOffset {
		m.viewport.YOffset = maxOffset
	}
}

func getClipboardCommand() (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pbcopy"), nil
	case "linux":
		return exec.Command("xclip", "-selection", "clipboard"), nil
	case "windows":
		return exec.Command("clip"), nil
	default:
		return nil, fmt.Errorf("unsupported platform for clipboard operations")
	}
}

func main() {
	// Add version flag
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("gpt-term version %s\n", version)
		os.Exit(0)
	}

	if os.Getenv("CLAUDE_API_KEY") == "" {
		fmt.Println("Error: CLAUDE_API_KEY environment variable is not defined")
		os.Exit(1)
	}

	m, err := initialModel()
	if err != nil {
		fmt.Printf("Error initializing model: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
