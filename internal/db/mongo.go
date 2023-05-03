/*
Copyright (C) 2021-2023, Kubefirst

This program is licensed under MIT.
See the LICENSE file for more details.
*/
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kubefirst/kubefirst-api/internal/objectStorage"
	"github.com/kubefirst/kubefirst-api/internal/types"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	clusterExportsPath = "/tmp/api/cluster/export"
	clusterImportsPath = "/tmp/api/cluster/import"
)

type MongoDBClient struct {
	Client             *mongo.Client
	ClustersCollection *mongo.Collection
	Context            context.Context
}

var Client = Connect()

// Connect
func Connect() *MongoDBClient {
	var connString string
	var clientOptions *options.ClientOptions

	ctx := context.Background()

	switch os.Getenv("MONGODB_HOST_TYPE") {
	case "atlas":
		serverAPI := options.ServerAPI(options.ServerAPIVersion1)
		connString = fmt.Sprintf("mongodb+srv://%s:%s@%s",
			os.Getenv("MONGODB_USERNAME"),
			os.Getenv("MONGODB_PASSWORD"),
			os.Getenv("MONGODB_HOST"),
		)
		clientOptions = options.Client().ApplyURI(connString).SetServerAPIOptions(serverAPI)
	case "local":
		connString = fmt.Sprintf("mongodb://%s:%s@%s/?authSource=admin",
			os.Getenv("MONGODB_USERNAME"),
			os.Getenv("MONGODB_PASSWORD"),
			os.Getenv("MONGODB_HOST"),
		)
		clientOptions = options.Client().ApplyURI(connString)
	}

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("could not create mongodb client: %s", err)
	}

	cl := MongoDBClient{
		Client:             client,
		ClustersCollection: client.Database("api").Collection("clusters"),
		Context:            ctx,
	}

	return &cl
}

// TestDatabaseConnection
func (mdbcl *MongoDBClient) TestDatabaseConnection() error {
	err := mdbcl.Client.Database("admin").RunCommand(mdbcl.Context, bson.D{{"ping", 1}}).Err()
	if err != nil {
		log.Fatalf("error connecting to mongodb: %s", err)
	}
	log.Infof("connected to mongodb host %s", os.Getenv("MONGODB_HOST"))

	return nil
}

// CRUD

// DeleteCluster
func (mdbcl *MongoDBClient) DeleteCluster(clusterName string) error {
	// Find
	filter := bson.D{{"cluster_name", clusterName}}

	// Delete
	resp, err := mdbcl.ClustersCollection.DeleteOne(mdbcl.Context, filter)
	if err != nil {
		return fmt.Errorf("error deleting cluster %s: %s", clusterName, err)
	}

	log.Infof("cluster deleted: %v", resp.DeletedCount)

	return nil
}

// GetCluster
func (mdbcl *MongoDBClient) GetCluster(clusterName string) (types.Cluster, error) {
	// Find
	filter := bson.D{{"cluster_name", clusterName}}
	var result types.Cluster
	err := mdbcl.ClustersCollection.FindOne(mdbcl.Context, filter).Decode(&result)
	if err != nil {
		return types.Cluster{}, fmt.Errorf("error getting cluster %s: %s", clusterName, err)
	}

	return result, nil
}

// GetClusters
func (mdbcl *MongoDBClient) GetClusters() ([]types.Cluster, error) {
	// Find all
	var results []types.Cluster
	cursor, err := mdbcl.ClustersCollection.Find(mdbcl.Context, bson.D{})
	if err != nil {
		return []types.Cluster{}, fmt.Errorf("error getting clusters: %s", err)
	}

	for cursor.Next(mdbcl.Context) {
		//Create a value into which the single document can be decoded
		var cl types.Cluster
		err := cursor.Decode(&cl)
		if err != nil {
			return []types.Cluster{}, err
		}
		results = append(results, cl)

	}
	if err := cursor.Err(); err != nil {
		return []types.Cluster{}, err
	}

	cursor.Close(mdbcl.Context)

	return results, nil
}

// InsertCluster
func (mdbcl *MongoDBClient) InsertCluster(cl types.Cluster) error {
	filter := bson.D{{"cluster_name", cl.ClusterName}}
	var result types.Cluster
	err := mdbcl.ClustersCollection.FindOne(mdbcl.Context, filter).Decode(&result)
	if err != nil {
		// This error means your query did not match any documents.
		if err == mongo.ErrNoDocuments {
			// Create if entry does not exist
			insert, err := mdbcl.ClustersCollection.InsertOne(mdbcl.Context, cl)
			if err != nil {
				return fmt.Errorf("error inserting cluster %s: %s", cl.ClusterName, err)
			}
			log.Info(insert)
		}
	} else {
		log.Infof("cluster record for %s already exists - skipping", cl.ClusterName)
	}

	return nil
}

// UpdateCluster
func (mdbcl *MongoDBClient) UpdateCluster(clusterName string, field string, value interface{}) error {
	// Find
	filter := bson.D{{"cluster_name", clusterName}}
	var result types.Cluster
	err := mdbcl.ClustersCollection.FindOne(mdbcl.Context, filter).Decode(&result)
	if err != nil {
		return fmt.Errorf("error finding cluster %s: %s", clusterName, err)
	}

	// Update
	filter = bson.D{{"_id", result.ID}}
	update := bson.D{{"$set", bson.D{{field, value}}}}
	resp, err := mdbcl.ClustersCollection.UpdateOne(mdbcl.Context, filter, update)
	if err != nil {
		return fmt.Errorf("error updating cluster %s: %s", clusterName, err)
	}

	log.Infof("cluster updated: %v", resp.ModifiedCount)

	return nil
}

// Backups

// Export parses the contents of a single cluster to a local file
func (mdbcl *MongoDBClient) Export(clusterName string) error {
	var localFilePath = fmt.Sprintf("%s/%s.json", clusterExportsPath, clusterName)
	var remoteFilePath = fmt.Sprintf("%s.json", clusterName)

	// Find
	filter := bson.D{{"cluster_name", clusterName}}
	var result types.Cluster
	err := mdbcl.ClustersCollection.FindOne(mdbcl.Context, filter).Decode(&result)
	if err != nil {
		return err
	}

	// Format file for cluster dump
	// Verify export directory exists
	if _, err := os.Stat(clusterExportsPath); os.IsNotExist(err) {
		log.Info("cluster exports directory does not exist, creating")
		err := os.MkdirAll(clusterExportsPath, 0777)
		if err != nil {
			return err
		}
	}

	file, _ := json.MarshalIndent(result, "", " ")
	_ = os.WriteFile(localFilePath, file, 0644)

	// Put file containing cluster dump in object storage
	err = objectStorage.PutClusterObject(
		&result.StateStoreCredentials,
		&result.StateStoreDetails,
		&types.PushBucketObject{
			LocalFilePath:  localFilePath,
			RemoteFilePath: remoteFilePath,
			ContentType:    "application/json",
		},
	)
	if err != nil {
		return err
	}

	log.Infof("successfully exported cluster %s to object storage bucket", clusterName)

	err = os.Remove(localFilePath)
	if err != nil {
		return err
	}

	return nil
}

// Restore retrieves a target cluster's database export from a state storage bucket and
// attempts to insert the parsed cluster object into the database
func (mdbcl *MongoDBClient) Restore(req *types.ImportClusterRequest) error {
	var cluster *types.Cluster
	var localFilePath = fmt.Sprintf("%s/%s.json", clusterImportsPath, req.ClusterName)
	var remoteFilePath = fmt.Sprintf("%s.json", req.ClusterName)

	// Verify import directory exists
	if _, err := os.Stat(clusterImportsPath); os.IsNotExist(err) {
		log.Info("cluster imports directory does not exist, creating")
		err := os.MkdirAll(clusterImportsPath, 0777)
		if err != nil {
			return err
		}
	}

	// Retrieve the object from state storage bucket
	err := objectStorage.GetClusterObject(
		&types.StateStoreCredentials{
			AccessKeyID:     req.StateStoreCredentials.AccessKeyID,
			SecretAccessKey: req.StateStoreCredentials.SecretAccessKey,
		},
		// todo: to support AWS, additional fields are required
		// AWSStateStoreBucket
		// AWSArtifactsBucket
		&types.StateStoreDetails{
			Name:     req.StateStoreDetails.Name,
			Hostname: req.StateStoreDetails.Hostname,
		},
		req.ClusterName,
		localFilePath,
		remoteFilePath,
	)
	if err != nil {
		return err
	}

	// Marshal file contents into cluster struct
	fileContents, err := os.ReadFile(localFilePath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(fileContents, &cluster)
	if err != nil {
		return fmt.Errorf("target file %s does not appear to be valid json: %s", localFilePath, err)
	}

	// Insert the cluster into the target database
	err = mdbcl.InsertCluster(*cluster)
	if err != nil {
		return err
	}

	log.Infof("successfully restored cluster %s to database", req.ClusterName)

	err = os.Remove(localFilePath)
	if err != nil {
		return err
	}

	return nil
}
