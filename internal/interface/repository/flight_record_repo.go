package repository

import (
	"context"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoFlightRecordRepository implements FlightRecordRepository
type MongoFlightRecordRepository struct {
	collection *mongo.Collection
}

// NewMongoFlightRecordRepository creates a new flight record repository
func NewMongoFlightRecordRepository(db *mongo.Database) repository.FlightRecordRepository {
	collection := db.Collection("flight_records")

	// Create unique index on bookingKey
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"bookingKey": 1},
		Options: options.Index().SetUnique(true),
	}
	collection.Indexes().CreateOne(ctx, indexModel)

	// Create index on providerPnr for queries
	pnrIndex := mongo.IndexModel{
		Keys: bson.M{"providerPnr": 1},
	}
	collection.Indexes().CreateOne(ctx, pnrIndex)

	return &MongoFlightRecordRepository{
		collection: collection,
	}
}

// FindByBookingKey finds a flight record by booking key
func (r *MongoFlightRecordRepository) FindByBookingKey(ctx context.Context, bookingKey string) (*entity.FlightRecord, error) {
	var record entity.FlightRecord
	err := r.collection.FindOne(ctx, bson.M{"bookingKey": bookingKey}).Decode(&record)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// Upsert creates or updates a flight record
func (r *MongoFlightRecordRepository) Upsert(ctx context.Context, record *entity.FlightRecord) error {
	if record.ID == "" {
		record.ID = primitive.NewObjectID().Hex()
		record.CreatedAt = time.Now()
	}
	record.UpdatedAt = time.Now()

	opts := options.Replace().SetUpsert(true)
	_, err := r.collection.ReplaceOne(
		ctx,
		bson.M{"bookingKey": record.BookingKey},
		record,
		opts,
	)
	return err
}

// UpdateTaskInfo updates task tracking information
func (r *MongoFlightRecordRepository) UpdateTaskInfo(ctx context.Context, bookingKey string, taskID string, scheduledAt time.Time) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"bookingKey": bookingKey},
		bson.M{"$set": bson.M{
			"lastTaskId":      taskID,
			"lastScheduledAt": scheduledAt,
			"updatedAt":       time.Now(),
		}},
	)
	return err
}
