package persistence

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewMongoClient creates a new MongoDB client and returns it with the database
func NewMongoClient(ctx context.Context, uri, dbname string) (*mongo.Client, *mongo.Database, error) {
	clientOptions := options.Client().ApplyURI(uri)

	// Only set auth if username and password are provided and not already in URI
	// if username != "" && password != "" && !strings.Contains(uri, "@") {
	// 	clientOptions.SetAuth(options.Credential{
	// 		Username: username,
	// 		Password: password,
	// 	})
	// }

	// Set connection timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, nil, err
	}

	// Ping to check connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	db := client.Database(dbname)

	return client, db, nil
}

// GetDatabase gets a database from the client
func GetDatabase(client *mongo.Client, name string) *mongo.Database {
	return client.Database(name)
}
