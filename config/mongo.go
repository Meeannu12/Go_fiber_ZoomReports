package config

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var MongoClient *mongo.Client

func ConnectMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		panic(err)
	}

	MongoClient = client
	fmt.Println("✅ MongoDB connected successfully!")
}

func GetCollection(dbName, collectionName string) *mongo.Collection {
	return MongoClient.Database(dbName).Collection(collectionName)
}

// var DB *mongo.Database

// func ConnectMongoDB() {
// 	uri := "mongodb://localhost:27017"

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	// Connect to MongoDB
// 	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
// 	if err != nil {
// 		log.Fatal("MongoDb Connection Error: ", err)
// 	}
// 	// Ping to verify connection
// 	err = client.Ping(ctx, nil)
// 	if err != nil {
// 		log.Fatal("Could not Ping MongoDB: ", err)
// 	}

// 	// Select database name
// 	DB = client.Database("ZoomDB")

// 	fmt.Println("Connection to Mongodb Successfully")

// }
