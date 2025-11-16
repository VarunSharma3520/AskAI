package vector

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/VarunSharma3520/AskAI/internal/logger"
	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultQdrantAddress = "localhost:6333"
	defaultCollection    = "askai_questions"
	vectorSize           = 1024 // Default vector size, adjust based on your embedder
)

// Embedder defines the interface for text embedding models
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// VectorStore handles storing and retrieving vectors from Qdrant
type VectorStore struct {
	collectionsClient pb.CollectionsClient
	pointsClient      pb.PointsClient
	collection        string
	embedder          Embedder
	logger            *logger.Logger
}

// NewVectorStore creates a new VectorStore instance
func NewVectorStore(conn *grpc.ClientConn, collection string, embedder Embedder, logger *logger.Logger) *VectorStore {
	return &VectorStore{
		collectionsClient: pb.NewCollectionsClient(conn),
		pointsClient:      pb.NewPointsClient(conn),
		collection:        collection,
		embedder:          embedder,
		logger:            logger,
	}
}

// Embed creates a vector embedding for the given text
func (vs *VectorStore) Embed(text string) ([]float32, error) {
	if vs.embedder == nil {
		return nil, fmt.Errorf("no embedder configured")
	}
	return vs.embedder.Embed(text)
}

// EnsureCollection creates the collection if it doesn't exist
func (vs *VectorStore) EnsureCollection(vectorSize uint64) error {
	vs.logger.Info(fmt.Sprintf("Ensuring collection '%s' exists with vector size %d", vs.collection, vectorSize), nil)

	// First check if collection exists
	_, err := vs.collectionsClient.Get(context.Background(), &pb.GetCollectionInfoRequest{
		CollectionName: vs.collection,
	})

	// If collection doesn't exist, create it
	if err != nil {
		vs.logger.Info(fmt.Sprintf("Collection '%s' does not exist, creating it...", vs.collection), nil)

		_, createErr := vs.collectionsClient.Create(context.Background(), &pb.CreateCollection{
			CollectionName: vs.collection,
			VectorsConfig: &pb.VectorsConfig{
				Config: &pb.VectorsConfig_Params{
					Params: &pb.VectorParams{
						Size:     vectorSize,
						Distance: pb.Distance_Cosine,
					},
				},
			},
		})

		if createErr != nil {
			errMsg := fmt.Sprintf("failed to create collection '%s'", vs.collection)
			vs.logger.Error(errMsg, createErr, nil)
			return fmt.Errorf("%s: %w", errMsg, createErr)
		}

		vs.logger.Info(fmt.Sprintf("Created collection '%s' with vector size %d", vs.collection, vectorSize), nil)
	} else {
		vs.logger.Info(fmt.Sprintf("Collection '%s' already exists, skipping creation", vs.collection), nil)
	}

	return nil
}

// ResetIndex deletes and recreates the collection to reset the index
func (vs *VectorStore) ResetIndex() error {
	// First delete the collection if it exists
	_, err := vs.collectionsClient.Delete(context.Background(), &pb.DeleteCollection{
		CollectionName: vs.collection,
	})

	// Ignore error if collection doesn't exist
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	// Recreate the collection
	return vs.EnsureCollection(vectorSize)
}

// isNotFoundError checks if the error is a "not found" error from Qdrant
func isNotFoundError(err error) bool {
	// This is a simple check, you might need to adjust based on the actual error from Qdrant
	return err != nil && (err.Error() == "not found" || err.Error() == "collection not found")
}

// StoreVector stores a vector with metadata in Qdrant
func (vs *VectorStore) StoreVector(id string, vector []float32, metadata map[string]string) error {
	vs.logger.Info(fmt.Sprintf("Preparing to store vector in collection '%s' with ID: %s", vs.collection, id), nil)

	// Convert metadata to Qdrant's Value format
	payload := make(map[string]*pb.Value)
	for k, v := range metadata {
		payload[k] = &pb.Value{
			Kind: &pb.Value_StringValue{
				StringValue: v,
			},
		}
	}

	// Prepare point with ID, vector, and payload
	point := &pb.PointStruct{
		Id: &pb.PointId{
			PointIdOptions: &pb.PointId_Uuid{
				Uuid: id,
			},
		},
		Vectors: &pb.Vectors{
			VectorsOptions: &pb.Vectors_Vector{
				Vector: &pb.Vector{
					Data: vector,
				},
			},
		},
		Payload: payload,
	}

	// Log the upsert operation with context
	vs.logger.Info("sending upsert request to Qdrant",
		map[string]interface{}{"point_id": id, "collection": vs.collection})

	// Upsert the point
	_, err := vs.pointsClient.Upsert(context.Background(), &pb.UpsertPoints{
		CollectionName: vs.collection,
		Points:         []*pb.PointStruct{point},
	})

	if err != nil {
		vs.logger.Error("failed to store vector in Qdrant",
			err,
			map[string]interface{}{
				"collection": vs.collection,
				"point_id":   id,
			})
		return fmt.Errorf("failed to store vector in Qdrant: %w", err)
	}

	vs.logger.Info("successfully stored vector in Qdrant",
		map[string]interface{}{"point_id": id, "collection": vs.collection})
	return nil
}

// StoreQA stores a question and its answer in Qdrant with proper metadata
func (vs *VectorStore) StoreQA(question, answer string, questionEmbedding, answerEmbedding []float32) error {
	// First check if this Q&A pair already exists
	exists, err := vs.QAExists(question, answer)
	if err != nil {
		vs.logger.Error("error searching for similar questions", err, nil)
		return fmt.Errorf("failed to check for existing Q&A: %w", err)
	}

	if exists {
		vs.logger.Info("Q&A pair already exists in Qdrant",
			map[string]interface{}{"question": question})
		return nil
	}

	vs.logger.Info("Storing Q&A", map[string]interface{}{
		"question":             question,
		"question_vector_size": len(questionEmbedding),
		"answer_vector_size":   len(answerEmbedding),
	})

	vs.logger.Info(fmt.Sprintf("Storing vector with ID: %s, vector size: %d", uuid.New().String(), len(questionEmbedding)), nil)

	// Create a single ID for the Q&A pair
	qaID := uuid.New().String()

	// Get current timestamp
	timestamp := time.Now().Format(time.RFC3339)

	// Store a single vector with combined Q&A information
	err = vs.StoreVector(
		qaID,
		questionEmbedding, // Using question embedding for search
		map[string]string{
			"type":        "qa_pair",
			"question":    question,
			"answer":      answer,
			"stored_at":   timestamp,
			"vector_type": "question",
		},
	)

	if err != nil {
		vs.logger.Error("error storing vector", err, nil)
		return fmt.Errorf("failed to store Q&A: %w", err)
	}

	vs.logger.Info(fmt.Sprintf("Successfully stored Q&A in Qdrant. ID: %s", qaID), nil)
	return nil
}

// SearchSimilarQuestions finds similar questions in Qdrant
func (vs *VectorStore) SearchSimilarQuestions(question string, limit int32) ([]*pb.ScoredPoint, error) {
	// Generate embedding for the question
	embedding, err := vs.Embed(question)
	if err != nil {
		return nil, fmt.Errorf("failed to embed question: %w", err)
	}

	// Prepare search request
	result, err := vs.pointsClient.Search(context.Background(), &pb.SearchPoints{
		CollectionName: vs.collection,
		Vector:         embedding,
		Limit:          uint64(limit),
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
		Filter: &pb.Filter{
			Should: []*pb.Condition{
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key: "type",
							Match: &pb.Match{
								MatchValue: &pb.Match_Keyword{
									Keyword: "qa_pair",
								},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return result.Result, nil
}

// ConnectToQdrant creates a new gRPC connection to Qdrant server
func ConnectToQdrant(address string) (*grpc.ClientConn, error) {
	if address == "" {
		address = defaultQdrantAddress
	}

	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	return conn, nil
}

// NewDefaultVectorStore creates a new VectorStore with default settings
func NewDefaultVectorStore(embedder Embedder, vaultPath string) (*VectorStore, error) {
	// Initialize logger
	logPath := filepath.Join(vaultPath, "log.json")
	log, err := logger.GetLogger(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Connect to Qdrant
	conn, err := ConnectToQdrant(defaultQdrantAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	// Create a new vector store
	vs := NewVectorStore(conn, defaultCollection, embedder, log)

	// Ensure the collection exists
	if err := vs.EnsureCollection(vectorSize); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	return vs, nil
}

// QAExists checks if a Q&A pair already exists in Qdrant
func (vs *VectorStore) QAExists(question, answer string) (bool, error) {
	// Generate embedding for the question
	embedding, err := vs.Embed(question)
	if err != nil {
		return false, fmt.Errorf("failed to embed question: %w", err)
	}

	// Search for similar questions
	searchResult, err := vs.pointsClient.Search(context.Background(), &pb.SearchPoints{
		CollectionName: vs.collection,
		Vector:         embedding,
		Limit:          5, // Check top 5 most similar
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
		Filter: &pb.Filter{
			Must: []*pb.Condition{
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key: "type",
							Match: &pb.Match{
								MatchValue: &pb.Match_Keyword{
									Keyword: "qa_pair",
								},
							},
						},
					},
				},
			},
		},
	})

	if err != nil {
		return false, fmt.Errorf("search failed: %w", err)
	}

	// Check if any result matches both question and answer
	for _, result := range searchResult.GetResult() {
		payload := result.GetPayload()
		if payload == nil {
			continue
		}

		// Get stored question and answer
		storedQ, hasQ := payload["question"]
		storedA, hasA := payload["answer"]

		if hasQ && hasA && storedQ.GetStringValue() == question && storedA.GetStringValue() == answer {
			return true, nil
		}
	}

	return false, nil
}

// SearchSimilar finds similar vectors in Qdrant
func (vs *VectorStore) SearchSimilar(vector []float32, limit uint32) ([]*pb.ScoredPoint, error) {
	// Prepare search request
	sc, err := vs.pointsClient.Search(context.Background(), &pb.SearchPoints{
		CollectionName: vs.collection,
		Vector:         vector,
		Limit:          uint64(limit), // Convert uint32 to uint64
	})

	if err != nil {
		vs.logger.Error("Failed to search vectors", err, nil)
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return sc.Result, nil
}
