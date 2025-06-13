package persistence

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewMongoClient creates a new MongoDB client
func NewMongoClient(ctx context.Context, uri, username, password string) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(uri)
	
	if username != "" && password != "" {
		clientOptions.SetAuth(options.Credential{
			Username: username,
			Password: password,
		})
	}
	
	// Set connection timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}
	
	// Ping to check connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	
	return client, nil
}

// GetDatabase gets a database from the client
func GetDatabase(client *mongo.Client, name string) *mongo.Database {
	return client.Database(name)
}