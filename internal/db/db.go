// db/qdrant.go
package db

import (
	"context"
	"log"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// QdrantClient wraps the Qdrant client with our custom methods
type QdrantClient struct {
	client     qdrant.CollectionsClient
	points     qdrant.PointsClient
	collection string
}

// InitQdrant initializes a new Qdrant client
func InitQdrant(collection string, vectorSize uint64) *QdrantClient {
	// Create a gRPC connection
	conn, err := grpc.Dial("localhost:6334", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}

	return &QdrantClient{
		client:     qdrant.NewCollectionsClient(conn),
		points:     qdrant.NewPointsClient(conn),
		collection: collection,
	}
}

// StoreVector stores a vector with metadata in Qdrant
func (qc *QdrantClient) StoreVector(id string, vector []float32, metadata map[string]string) error {
	// Convert metadata to Qdrant's Value format
	payload := make(map[string]*qdrant.Value)
	for k, v := range metadata {
		payload[k] = &qdrant.Value{
			Kind: &qdrant.Value_StringValue{
				StringValue: v,
			},
		}
	}

	// Create point with ID, vector, and payload
	point := &qdrant.PointStruct{
		Id: &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Uuid{
				Uuid: id,
			},
		},
		Vectors: &qdrant.Vectors{
			VectorsOptions: &qdrant.Vectors_Vector{
				Vector: &qdrant.Vector{
					Data: vector,
				},
			},
		},
		Payload: payload,
	}

	// Store the point in Qdrant
	_, err := qc.points.Upsert(context.Background(), &qdrant.UpsertPoints{
		CollectionName: qc.collection,
		Points:         []*qdrant.PointStruct{point},
	})

	if err != nil {
		// log.Printf("Failed to store vector: %v", err)
		return err
	}

	return nil
}
