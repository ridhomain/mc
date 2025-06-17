package repository

import (
	"context"
	"fmt"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoEmailRepository implements the EmailRepository interface
type MongoEmailRepository struct {
	collection *mongo.Collection
}

// NewMongoEmailRepository creates a new MongoDB email repository
func NewMongoEmailRepository(db *mongo.Database) repository.EmailRepository {
	collection := db.Collection("emails")

	// Create indexes for better performance
	ctx := context.Background()

	// Index on emailID for fast lookups and uniqueness
	emailIDIndex := mongo.IndexModel{
		Keys:    bson.M{"emailid": 1},
		Options: options.Index().SetUnique(true),
	}

	// Index on processstatus for finding emails by status
	processStatusIndex := mongo.IndexModel{
		Keys: bson.M{"processstatus": 1},
	}

	// Index on receivedAt for sorting and filtering
	receivedAtIndex := mongo.IndexModel{
		Keys: bson.M{"receivedat": -1},
	}

	// Compound index for finding unprocessed emails efficiently
	unprocessedIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "processstatus", Value: 1},
			{Key: "receivedat", Value: 1},
		},
	}

	// Create all indexes
	collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		emailIDIndex,
		processStatusIndex,
		receivedAtIndex,
		unprocessedIndex,
	})

	return &MongoEmailRepository{
		collection: collection,
	}
}

// Save saves an email to MongoDB
func (r *MongoEmailRepository) Save(ctx context.Context, email *entity.Email) error {
	// if email.ID == "" {
	// 	email.ID = primitive.NewObjectID().Hex()
	// }

	if email.ProcessStatus == "" {
		email.ProcessStatus = entity.StatusPending
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

// FindUnprocessed finds unprocessed emails (PENDING status or empty)
func (r *MongoEmailRepository) FindUnprocessed(ctx context.Context, limit int) ([]*entity.Email, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"processstatus": ""},
			{"processstatus": entity.StatusPending},
			{"processstatus": bson.M{"$exists": false}},
		},
	}

	limit64 := int64(limit)
	cursor, err := r.collection.Find(ctx, filter, &options.FindOptions{
		Limit: &limit64,
		Sort:  bson.D{{Key: "receivedat", Value: 1}}, // Process oldest first
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

// UpdateStatus updates just the status and started time
func (r *MongoEmailRepository) UpdateStatus(ctx context.Context, id string, status string, startedAt time.Time) error {
	update := bson.M{
		"$set": bson.M{
			"processstatus": status,
		},
	}

	// Only set processstartedat when moving to PROCESSING
	if status == entity.StatusProcessing && !startedAt.IsZero() {
		update["$set"].(bson.M)["processstartedat"] = startedAt
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		update,
	)

	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no document found with id: %s", id)
	}

	return nil
}

// UpdateProcessSteps updates the processing steps
func (r *MongoEmailRepository) UpdateProcessSteps(ctx context.Context, id string, steps entity.ProcessSteps) error {
	update := bson.M{
		"$set": bson.M{
			"processsteps": steps,
		},
	}

	res, err := r.collection.UpdateOne(
		ctx,
		bson.M{"id": id},
		update,
	)
	fmt.Printf("Update result: %v\n", res)
	return err
}

// MarkAsProcessed marks an email as processed with full details
func (r *MongoEmailRepository) MarkAsProcessed(ctx context.Context, id, status, processorType, errorDetail string, extractedData map[string]interface{}) error {
	update := bson.M{
		"$set": bson.M{
			"processedat":   time.Now(),
			"processstatus": status,
			"processortype": processorType,
		},
	}

	if extractedData != nil && len(extractedData) > 0 {
		update["$set"].(bson.M)["extracteddata"] = extractedData
	}

	if errorDetail != "" {
		update["$set"].(bson.M)["errordetail"] = errorDetail
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		update,
	)

	if err != nil {
		return fmt.Errorf("failed to mark as processed: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no document found with id: %s", id)
	}

	return nil
}

// ResetProcessingEmails resets emails stuck in PROCESSING state back to PENDING
func (r *MongoEmailRepository) ResetProcessingEmails(ctx context.Context) error {
	// Find emails that have been processing for more than 5 minutes
	staleTime := time.Now().Add(-5 * time.Minute)

	filter := bson.M{
		"processstatus": entity.StatusProcessing,
		"$or": []bson.M{
			{"processstartedat": bson.M{"$lt": staleTime}},
			{"processstartedat": bson.M{"$exists": false}},
		},
	}

	update := bson.M{
		"$set": bson.M{
			"processstatus": entity.StatusPending,
			"errordetail":   "Reset from stale PROCESSING state",
		},
	}

	result, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.ModifiedCount > 0 {
		// Log how many were reset
		fmt.Printf("Reset %d stale processing emails\n", result.ModifiedCount)
	}

	return nil
}

// FindByStatus finds emails by status
func (r *MongoEmailRepository) FindByStatus(ctx context.Context, status string, limit int) ([]*entity.Email, error) {
	filter := bson.M{"processstatus": status}

	limit64 := int64(limit)
	cursor, err := r.collection.Find(ctx, filter, &options.FindOptions{
		Limit: &limit64,
		Sort:  bson.D{{Key: "receivedat", Value: -1}}, // Most recent first
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

// GetLastEmail gets the most recently received email
func (r *MongoEmailRepository) GetLastEmail(ctx context.Context) (*entity.Email, error) {
	var email entity.Email
	opts := options.FindOne().SetSort(bson.D{{Key: "receivedat", Value: -1}})
	err := r.collection.FindOne(ctx, bson.M{}, opts).Decode(&email)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &email, nil
}

// Email IDs
// FindByEmailID finds an email by Gmail email ID
func (r *MongoEmailRepository) FindByEmailID(ctx context.Context, emailID string) (*entity.Email, error) {
	var email entity.Email
	err := r.collection.FindOne(ctx, bson.M{"emailid": emailID}).Decode(&email)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &email, nil
}

// FindByEmailIDs finds multiple emails by Gmail message IDs (batch operation)
func (r *MongoEmailRepository) FindByEmailIDs(ctx context.Context, emailIDs []string) (map[string]*entity.Email, error) {
	if len(emailIDs) == 0 {
		return make(map[string]*entity.Email), nil
	}

	filter := bson.M{"emailid": bson.M{"$in": emailIDs}}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make(map[string]*entity.Email)
	for cursor.Next(ctx) {
		var email entity.Email
		if err := cursor.Decode(&email); err != nil {
			continue
		}
		result[email.EmailID] = &email
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *MongoEmailRepository) UpdateStatusByEmailID(ctx context.Context, emailID string, status string, startedAt time.Time) error {
	update := bson.M{
		"$set": bson.M{
			"processstatus": status,
		},
	}

	if status == entity.StatusProcessing && !startedAt.IsZero() {
		update["$set"].(bson.M)["processstartedat"] = startedAt
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"emailid": emailID},
		update,
	)

	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no document found with emailID: %s", emailID)
	}

	return nil
}

func (r *MongoEmailRepository) MarkAsProcessedByEmailID(ctx context.Context, emailID, status, processorType, errorDetail string, extractedData map[string]interface{}) error {
	update := bson.M{
		"$set": bson.M{
			"processedat":   time.Now(),
			"processstatus": status,
			"processortype": processorType,
		},
	}

	if extractedData != nil && len(extractedData) > 0 {
		update["$set"].(bson.M)["extracteddata"] = extractedData
	}

	if errorDetail != "" {
		update["$set"].(bson.M)["errordetail"] = errorDetail
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"emailid": emailID},
		update,
	)

	if err != nil {
		return fmt.Errorf("failed to mark as processed: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no document found with emailID: %s", emailID)
	}

	return nil
}

func (r *MongoEmailRepository) UpdateProcessStepsByEmailID(ctx context.Context, emailID string, steps entity.ProcessSteps) error {
	update := bson.M{
		"$set": bson.M{
			"processsteps": steps,
		},
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"emailID": emailID},
		update,
	)
	return err
}
