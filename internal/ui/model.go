// Package ui provides the terminal user interface components for the AskAI application.
// This file defines the main application model and its core functionality.
package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/VarunSharma3520/AskAI/internal/config"
	"github.com/VarunSharma3520/AskAI/internal/types"
	"github.com/VarunSharma3520/AskAI/internal/vector"
	"github.com/charmbracelet/bubbles/textinput"
)

// Model represents the main application state and business logic.
// It manages the UI state, handles user input, and coordinates with the vector store.
type Model struct {
	TextInput    textinput.Model
	ModelInput   textinput.Model // For editing model name
	APIURLInput  textinput.Model // For editing API URL
	Msg          string
	ScreenMode   types.ScreenMode
	LastQuestion string
	VectorStore  *vector.VectorStore
	VaultPath    string

	// Options
	Options     []string
	SelectedOpt int
	ModelName   string
	Temperature float64
	MaxTokens   int

	// Stream handling
	StreamCh      chan string
	ErrCh         chan error
	StopCh        chan struct{}
	Streaming     bool
	StatusMsg     string
	StatusTimer   *time.Timer
	EditingModel  bool
	EditingAPIURL bool
}

// InitialModel creates and initializes a new Model instance with the provided vector store and vault path.
// It sets up the initial state of the application including the text input component and default values.
//
// Parameters:
//   - vectorStore: An initialized vector store instance for handling embeddings
//   - vaultPath: Filesystem path where application data will be stored
//
// Returns:
//   - *Model: A pointer to the newly created and initialized Model instance
//
// Example:
//
//	store := // initialize vector store
//	model := InitialModel(store, "/path/to/vault")
func InitialModel(vectorStore *vector.VectorStore, vaultPath string) *Model {
	time.Sleep(1 * time.Second)

	ti := textinput.New()
	ti.Placeholder = "Ask me anything..."
	ti.Focus()
	ti.CharLimit = 2000
	ti.Width = 80

	// Initialize model input
	modelInput := textinput.New()
	modelInput.Placeholder = "Enter model name (e.g., gemma3:1b)"
	modelInput.CharLimit = 50
	modelInput.Width = 30
	modelInput.Prompt = "> "

	// Initialize API URL input
	apiURLInput := textinput.New()
	apiURLInput.Placeholder = "Enter API URL (e.g., http://localhost:11434)"
	apiURLInput.CharLimit = 200
	apiURLInput.Width = 50
	apiURLInput.Prompt = "> "

	// Load saved settings or use defaults
	modelName := config.Model()
	temperature := config.Temperature()

	// Initialize options
	options := []string{
		"Change Model: " + modelName,
		fmt.Sprintf("Temperature: %.1f (use +/-", temperature),
		"Set API URL: " + config.APIURL(),
		"Save Settings",
		"Back to Chat",
		"Update Qdrant index",
	}

	return &Model{
		TextInput:     ti,
		ModelInput:    modelInput,
		APIURLInput:   apiURLInput,
		VectorStore:   vectorStore,
		VaultPath:     vaultPath,
		ScreenMode:    types.ModeChat,
		Options:       options,
		SelectedOpt:   0,
		ModelName:     modelName,
		Temperature:   temperature,
		EditingModel:  false,
		EditingAPIURL: false,
		StatusTimer:   time.NewTimer(0), // Will be reset when used
	}
}

// QA represents a single question-answer pair with metadata.
// It's used for both in-memory representation and JSON serialization.
type QA struct {
	Question string    `json:"question"` // The user's question
	Answer   string    `json:"answer"`   // The AI's response
	Time     time.Time `json:"time"`     // When the Q&A was created
}

// QAFile represents the structure of the saved Q&A data file.
// It's used to marshal and unmarshal Q&A pairs to/from JSON.
type QAFile struct {
	QAs []QA `json:"qas"` // Collection of Q&A pairs
}

// StoreQA saves a question-answer pair to the vault and creates vector embeddings for both.
// It performs the following operations:
// 1. Checks if the Q&A pair already exists in the vector store
// 2. Creates vector embeddings for both question and answer
// 3. Stores the Q&A in the local vault as JSON
// 4. Updates the vector store with the new embeddings
//
// Parameters:
//   - question: The user's question
//   - answer: The AI's response to the question
//
// Returns:
//   - error: An error if any step fails, or nil on success
//
// Example:
//
//	err := model.StoreQA("What is AI?", "AI stands for Artificial Intelligence.")
//	if err != nil {
//	    log.Printf("Failed to store Q&A: %v", err)
//	}
func (m *Model) StoreQA(question, answer string) error {
	if m.VectorStore == nil {
		return fmt.Errorf("vector store is not initialized")
	}

	// First check if Q&A already exists in Qdrant
	exists, err := m.VectorStore.QAExists(question, answer)
	if err != nil {
		// log.Printf("Error checking for existing Q&A in Qdrant: %v", err)
		// Continue with storage attempt even if check fails
	} else if exists {
		// log.Printf("Skipping duplicate Q&A pair in Qdrant: %s", question)
		return nil
	}

	// Create vault directory if it doesn't exist
	if err := os.MkdirAll(m.VaultPath, 0755); err != nil {
		return fmt.Errorf("failed to create vault directory: %w", err)
	}

	// Define the filename for the JSON storage
	filename := filepath.Join(m.VaultPath, "que_ans.json")
	var qas QAFile

	// Read existing Q&As if file exists
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read existing Q&A file: %w", err)
		}
		if err := json.Unmarshal(data, &qas); err != nil {
			return fmt.Errorf("failed to parse existing Q&A file: %w", err)
		}
	}

	// Generate embeddings for the question and answer
	questionEmbedding, err := m.VectorStore.Embed(question)
	if err != nil {
		log.Printf("Error embedding question: %v", err)
		return fmt.Errorf("failed to embed question: %w", err)
	}

	answerEmbedding, err := m.VectorStore.Embed(answer)
	if err != nil {
		log.Printf("Error embedding answer: %v", err)
		return fmt.Errorf("failed to embed answer: %w", err)
	}

	// Store both question and answer in Qdrant
	if err := m.VectorStore.StoreQA(question, answer, questionEmbedding, answerEmbedding); err != nil {
		log.Printf("Error storing Q&A in Qdrant: %v", err)
		return fmt.Errorf("failed to store Q&A in vector database: %w", err)
	}

	// log.Printf("Successfully stored Q&A pair in Qdrant")

	// Add new QA to the JSON file
	newQA := QA{
		Question: question,
		Answer:   answer,
		Time:     time.Now(),
	}
	qas.QAs = append(qas.QAs, newQA)

	// Convert to JSON
	data, err := json.MarshalIndent(qas, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Q&As: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write Q&As to file: %w", err)
	}

	return nil
}

// StoreCurrentQuestion indexes all Q&A pairs from the vault/que_ans.json file into Qdrant
func (m *Model) StoreCurrentQuestion() {
	// Define the filename for the JSON storage
	filename := filepath.Join(m.VaultPath, "que_ans.json")

	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		m.setStatus("No Q&A file found to index", 3*time.Second)
		log.Printf("Q&A file not found at: %s", filename)
		return
	}

	// Read the Q&A file
	data, err := os.ReadFile(filename)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to read Q&A file: %v", err)
		log.Println(errMsg)
		m.setStatus(errMsg, 5*time.Second)
		return
	}

	// Parse the Q&A data
	var qaFile QAFile
	if err := json.Unmarshal(data, &qaFile); err != nil {
		errMsg := fmt.Sprintf("Failed to parse Q&A file: %v", err)
		log.Println(errMsg)
		m.setStatus(errMsg, 5*time.Second)
		return
	}

	if len(qaFile.QAs) == 0 {
		m.setStatus("No Q&A pairs found to index", 3*time.Second)
		log.Println("No Q&A pairs found in the file")
		return
	}

	// Index each Q&A pair
	successCount := 0
	totalQAs := len(qaFile.QAs)
	m.setStatus(fmt.Sprintf("Starting to index %d Q&A pairs...", totalQAs), 0)

	for i, qa := range qaFile.QAs {
		if qa.Question == "" || qa.Answer == "" {
			log.Printf("Skipping empty Q&A at index %d", i)
			continue
		}

		// Update status
		progress := float64(i+1) / float64(totalQAs) * 100
		m.setStatus(fmt.Sprintf("Indexing %d/%d (%.1f%%)...", i+1, totalQAs, progress), 0)

		// Store the question and answer in the vector database
		if err := m.StoreQA(qa.Question, qa.Answer); err != nil {
			errMsg := fmt.Sprintf("Failed to index Q&A at index %d: %v", i, err)
			log.Println(errMsg)
			m.setStatus(errMsg, 3*time.Second)
			continue
		}
		successCount++
		// log.Printf("Successfully indexed Q&A %d/%d", i+1, totalQAs)
	}

	// Final status update
	if successCount > 0 {
		msg := fmt.Sprintf("✅ Successfully indexed %d/%d Q&A pairs", successCount, totalQAs)
		log.Println(msg)
		m.setStatus(msg, 10*time.Second)
	} else {
		errMsg := "❌ No valid Q&A pairs were indexed"
		log.Println(errMsg)
		m.setStatus(errMsg, 5*time.Second)
	}
}

// setStatus sets a status message that will be shown temporarily
func (m *Model) setStatus(msg string, duration time.Duration) {
	m.StatusMsg = msg
	if !m.StatusTimer.Stop() {
		select {
		case <-m.StatusTimer.C:
		default:
		}
	}
	m.StatusTimer.Reset(duration)
}

// ClearStatusIfExpired clears the status message if the timer has fired
func (m *Model) ClearStatusIfExpired() {
	select {
	case <-m.StatusTimer.C:
		m.StatusMsg = ""
	default:
		// Timer hasn't fired yet
	}
}
