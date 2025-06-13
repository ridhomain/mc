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

// MongoEmailRepository implements the EmailRepository interface
type MongoEmailRepository struct {
	collection *mongo.Collection
}

// NewMongoEmailRepository creates a new MongoDB email repository
func NewMongoEmailRepository(db *mongo.Database) repository.EmailRepository {
	return &MongoEmailRepository{
		collection: db.Collection("emails"),
	}
}

// Save saves an email to MongoDB
func (r *MongoEmailRepository) Save(ctx context.Context, email *entity.Email) error {
	if email.ID == "" {
		email.ID = primitive.NewObjectID().Hex()
	}

	_, err := r.collection.InsertOne(ctx, email)
	return err
}

// FindByID finds an email by ID
func (r *MongoEmailRepository) FindByID(ctx context.Context, id string) (*entity.Email, error) {
	var email entity.Email
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&email)
	if err != nil {
		return nil, err
	}
	return &email, nil
}

// FindUnprocessed finds unprocessed emails
func (r *MongoEmailRepository) FindUnprocessed(ctx context.Context, limit int) ([]*entity.Email, error) {
	filter := bson.M{"processedat": time.Time{}}

	limit64 := int64(limit)
	cursor, err := r.collection.Find(ctx, filter, &options.FindOptions{
		Limit: &limit64,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var emails []*entity.Email
	if err := cursor.All(ctx, &emails); err != nil {
		return nil, err
	}

	return emails, nil
}

// MarkAsProcessed marks an email as processed
func (r *MongoEmailRepository) MarkAsProcessed(ctx context.Context, id, status, processorType, errorDetail string, extractedData map[string]interface{}) error {
	update := bson.M{
		"$set": bson.M{
			"processedat":   time.Now(),
			"processstatus": status,
			"processortype": processorType,
		},
	}

	// Only add extractedData if it's not empty
	if extractedData != nil && len(extractedData) > 0 {
		update["$set"].(bson.M)["extracteddata"] = extractedData
	}

	// Only add errorDetail if it exists
	if errorDetail != "" {
		update["$set"].(bson.M)["errordetail"] = errorDetail
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		update,
	)
	return err
}

// MongoPayloadRepository implements the PayloadRepository interface
type MongoPayloadRepository struct {
	collection *mongo.Collection
}

// NewMongoPayloadRepository creates a new MongoDB payload repository
func NewMongoPayloadRepository(db *mongo.Database) repository.PayloadRepository {
	return &MongoPayloadRepository{
		collection: db.Collection("payloads"),
	}
}

// Save saves a payload to MongoDB
func (r *MongoPayloadRepository) Save(ctx context.Context, payload *entity.Payload) error {
	if payload.ID == "" {
		payload.ID = primitive.NewObjectID().Hex()
	}

	_, err := r.collection.InsertOne(ctx, payload)
	return err
}

// FindByID finds a payload by ID
func (r *MongoPayloadRepository) FindByID(ctx context.Context, id string) (*entity.Payload, error) {
	var payload entity.Payload
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&payload)
	if err != nil {
		return nil, err
	}
	return &payload, nil
}

// FindByStatus finds payloads by status
func (r *MongoPayloadRepository) FindByStatus(ctx context.Context, status string, limit int) ([]*entity.Payload, error) {
	filter := bson.M{"status": status}

	limit64 := int64(limit)
	cursor, err := r.collection.Find(ctx, filter, &options.FindOptions{
		Limit: &limit64,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var payloads []*entity.Payload
	if err := cursor.All(ctx, &payloads); err != nil {
		return nil, err
	}

	return payloads, nil
}

// UpdateStatus updates a payload's status
func (r *MongoPayloadRepository) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": status, "sentat": time.Now()}},
	)
	return err
}

func (r *MongoEmailRepository) FindByMessageID(ctx context.Context, messageID string) (*entity.Email, error) {
	var email entity.Email
	err := r.collection.FindOne(ctx, bson.M{"messageid": messageID}).Decode(&email)
	if err != nil {
		return nil, err
	}
	return &email, nil
}

func (r *MongoEmailRepository) GetLastEmail(ctx context.Context) (*entity.Email, error) {
	var email entity.Email
	opts := options.FindOne().SetSort(bson.D{{"receivedAt", -1}})
	err := r.collection.FindOne(ctx, bson.M{}, opts).Decode(&email)
	if err != nil {
		return nil, err
	}
	return &email, nil
}
