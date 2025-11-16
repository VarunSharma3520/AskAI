package main

import (
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/VarunSharma3520/AskAI/internal/config"
	"github.com/VarunSharma3520/AskAI/internal/fs"
	"github.com/VarunSharma3520/AskAI/internal/logger"
	"github.com/VarunSharma3520/AskAI/internal/ui"
	"github.com/VarunSharma3520/AskAI/internal/vector"
)

func main() {
	// Ensure vault exists before starting UI
	if err := fs.EnsureVaultExists(config.VaultPath()); err != nil {
		log.Fatalf("Failed to ensure vault folder exists: %v", err)
	}

	// Create a gRPC connection to Qdrant
	conn, err := grpc.NewClient("localhost:6334", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create Qdrant client: %v", err)
	}
	defer conn.Close()

	// Initialize the Ollama embedder
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	// Create Ollama embedder with mxbai-embed-large model
	embedder := vector.NewOllamaEmbedder(ollamaURL, "mxbai-embed-large")

	// Initialize logger
	logPath := filepath.Join(config.VaultPath(), "askai.log")
	log.Println("Initializing logger at:", logPath)
	log.Println("Vault path:", config.VaultPath())
	log.Println("Full log path:", logPath)

	// Create the log directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogger, err := logger.NewLogger(logPath)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Initialize vector store with the gRPC connection, embedder, and logger
	vectorStore := vector.NewVectorStore(conn, "askai_questions", embedder, appLogger)
	
	// Ensure the collection exists with the correct vector size
	// For mxbai-embed-large, the vector size is 1024
	vectorSize := uint64(1024)
	if err := vectorStore.EnsureCollection(vectorSize); err != nil {
		appLogger.Error("Failed to ensure Qdrant collection exists", err, nil)
		log.Fatalf("Failed to ensure Qdrant collection exists: %v", err)
	}

	// Get the vault path from config
	vaultPath := config.VaultPath()

	// Initialize UI with vector store and vault path
	p := tea.NewProgram(
		ui.InitialModel(vectorStore, vaultPath),
		tea.WithAltScreen(),
		tea.WithOutput(os.Stdout),
	)

	if _, err := p.Run(); err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}
