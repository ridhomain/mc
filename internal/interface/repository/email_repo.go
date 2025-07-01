// internal/interface/repository/email_repo.go
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
	collection := db.Collection("emailLogs")

	// Create indexes for better performance
	ctx := context.Background()

	emailIDIndex := mongo.IndexModel{
		Keys:    bson.M{"emailId": 1},
		Options: options.Index().SetUnique(true),
	}

	// Index on processStatus for finding emails by status
	processStatusIndex := mongo.IndexModel{
		Keys: bson.M{"processStatus": 1},
	}

	// Index on receivedAt for sorting and filtering
	receivedAtIndex := mongo.IndexModel{
		Keys: bson.M{"receivedAt": -1},
	}

	// Compound index for finding unprocessed emails efficiently
	unprocessedIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "processStatus", Value: 1},
			{Key: "receivedAt", Value: 1},
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
			{"processStatus": ""},
			{"processStatus": entity.StatusPending},
			{"processStatus": bson.M{"$exists": false}},
		},
	}

	limit64 := int64(limit)
	cursor, err := r.collection.Find(ctx, filter, &options.FindOptions{
		Limit: &limit64,
		Sort:  bson.D{{Key: "receivedAt", Value: 1}}, // Process oldest first
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
			"processStatus": status,
		},
	}

	// Only set processStartedAt when moving to PROCESSING
	if status == entity.StatusProcessing && !startedAt.IsZero() {
		update["$set"].(bson.M)["processStartedAt"] = startedAt
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
			"processSteps": steps,
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
			"processedAt":   time.Now(),
			"processStatus": status,
			"processorType": processorType,
		},
	}

	if extractedData != nil && len(extractedData) > 0 {
		update["$set"].(bson.M)["extractedData"] = extractedData
	}

	if errorDetail != "" {
		update["$set"].(bson.M)["errorDetail"] = errorDetail
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
		"processStatus": entity.StatusProcessing,
		"$or": []bson.M{
			{"processStartedAt": bson.M{"$lt": staleTime}},
			{"processStartedAt": bson.M{"$exists": false}},
		},
	}

	update := bson.M{
		"$set": bson.M{
			"processStatus": entity.StatusPending,
			"errorDetail":   "Reset from stale PROCESSING state",
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
	filter := bson.M{"processStatus": status}

	limit64 := int64(limit)
	cursor, err := r.collection.Find(ctx, filter, &options.FindOptions{
		Limit: &limit64,
		Sort:  bson.D{{Key: "receivedAt", Value: -1}}, // Most recent first
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
	opts := options.FindOne().SetSort(bson.D{{Key: "receivedAt", Value: -1}})
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
	err := r.collection.FindOne(ctx, bson.M{"emailId": emailID}).Decode(&email)
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

	filter := bson.M{"emailId": bson.M{"$in": emailIDs}}
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
			"processStatus": status,
		},
	}

	if status == entity.StatusProcessing && !startedAt.IsZero() {
		update["$set"].(bson.M)["processStartedAt"] = startedAt
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"emailId": emailID},
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
			"processedAt":   time.Now(),
			"processStatus": status,
			"processorType": processorType,
		},
	}

	if extractedData != nil && len(extractedData) > 0 {
		update["$set"].(bson.M)["extractedData"] = extractedData
	}

	if errorDetail != "" {
		update["$set"].(bson.M)["errorDetail"] = errorDetail
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"emailId": emailID},
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
			"processSteps": steps,
		},
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"emailId": emailID},
		update,
	)
	return err
}
