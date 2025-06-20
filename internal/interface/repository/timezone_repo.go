// internal/interface/repository/timezone_repo.go
package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoTimezoneRepository implements the TimezoneRepository interface
type MongoTimezoneRepository struct {
	collection *mongo.Collection
}

// NewMongoTimezoneRepository creates a new MongoDB timezone repository
func NewMongoTimezoneRepository(db *mongo.Database) repository.TimezoneRepository {
	collection := db.Collection("mtimezonelist")

	// Create index on airportCode for better performance
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"airportCode": 1},
		Options: options.Index().SetUnique(true),
	}
	collection.Indexes().CreateOne(ctx, indexModel)

	return &MongoTimezoneRepository{
		collection: collection,
	}
}

// GetByAirportCode finds a timezone by airport code
func (r *MongoTimezoneRepository) GetByAirportCode(ctx context.Context, code string) (*entity.Timezone, error) {
	var timezone entity.Timezone
	err := r.collection.FindOne(ctx, bson.M{"airportCode": code}).Decode(&timezone)
	if err != nil {
		return nil, err
	}
	return &timezone, nil
}

// GetTimezoneByCode retrieves a timezone by its code
func (r *MongoTimezoneRepository) GetTimezoneByCode(ctx context.Context, code string) (*entity.Timezone, error) {
	return r.GetByAirportCode(ctx, code)
}
