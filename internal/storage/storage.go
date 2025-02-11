package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Conversation struct {
	ID        string    `json:"id"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	Summary   string    `json:"summary"`
}

type Storage struct {
	baseDir string
}

func NewStorage() (*Storage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error getting home directory: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".gpt-term", "conversations")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating storage directory: %w", err)
	}

	return &Storage{baseDir: baseDir}, nil
}

func (s *Storage) SaveConversation(conv *Conversation) error {
	filename := fmt.Sprintf("%s_%s.convo",
		conv.CreatedAt.Format("2006-01-02T15-04-05"),
		conv.ID)

	filepath := filepath.Join(s.baseDir, filename)

	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling conversation: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("error writing conversation file: %w", err)
	}

	return nil
}

func (s *Storage) LoadConversation(id string) (*Conversation, error) {
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".convo" {
			data, err := os.ReadFile(filepath.Join(s.baseDir, file.Name()))
			if err != nil {
				continue
			}

			var conv Conversation
			if err := json.Unmarshal(data, &conv); err != nil {
				continue
			}

			if conv.ID == id {
				return &conv, nil
			}
		}
	}

	return nil, fmt.Errorf("conversation not found: %s", id)
}

func (s *Storage) ListConversations() ([]Conversation, error) {
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}

	var conversations []Conversation
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".convo" {
			data, err := os.ReadFile(filepath.Join(s.baseDir, file.Name()))
			if err != nil {
				continue
			}

			var conv Conversation
			if err := json.Unmarshal(data, &conv); err != nil {
				continue
			}

			conversations = append(conversations, conv)
		}
	}

	return conversations, nil
}

func (s *Storage) UpdateConversation(conv *Conversation) error {
	return s.SaveConversation(conv)
}

func (s *Storage) GenerateConversationSummary(messages []Message) string {
	if len(messages) == 0 {
		return "Empty conversation"
	}

	// Use the first user message as the summary
	for _, msg := range messages {
		if msg.Role == "user" {
			if len(msg.Content) > 50 {
				return msg.Content[:47] + "..."
			}
			return msg.Content
		}
	}

	return "No user messages"
}
