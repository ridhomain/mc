// internal/interface/repository/airline_repo.go
package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoAirlineRepository implements the AirlineRepository interface
type MongoAirlineRepository struct {
	collection *mongo.Collection
}

// NewMongoAirlineRepository creates a new MongoDB airline repository
func NewMongoAirlineRepository(db *mongo.Database) repository.AirlineRepository {
	collection := db.Collection("mairlines")

	// Create index on code for better performance
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"code": 1},
		Options: options.Index().SetUnique(true),
	}
	collection.Indexes().CreateOne(ctx, indexModel)

	return &MongoAirlineRepository{
		collection: collection,
	}
}

// GetByCode finds an airline by code
func (r *MongoAirlineRepository) GetByCode(ctx context.Context, code string) (*entity.Airline, error) {
	var airline entity.Airline
	err := r.collection.FindOne(ctx, bson.M{"code": code}).Decode(&airline)
	if err != nil {
		return nil, err
	}
	return &airline, nil
}
