package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStorage struct {
	client                  *mongo.Client
	categoriesCollection    *mongo.Collection
	subscriptionsCollection *mongo.Collection
}

func CreateMongoStorage() (*MongoStorage, error) {
	serverAPIOptions := options.ServerAPI(options.ServerAPIVersion1)
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(os.Getenv("MONGO")).SetServerAPIOptions(serverAPIOptions)) //("mongodb://localhost:27017"))
	if err != nil {
		return nil, err
	}
	fmt.Println(client)

	return &MongoStorage{
		client:                  client,
		categoriesCollection:    client.Database("dou").Collection("categories"),
		subscriptionsCollection: client.Database("dou").Collection("subscriptions"),
	}, nil
}

func (ms *MongoStorage) GetAllSubscribers(categoryId string) ([]SubscriptionInfo, error) {
	coll := ms.subscriptionsCollection
	filter := bson.D{{Key: "subscriptions.idCategory", Value: GetId(categoryId)}}
	res := []SubscriptionInfo{}
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}

	if err = cursor.All(context.TODO(), &res); err != nil {
		log.Fatal(err)
	}

	return res, nil
}
func (ms *MongoStorage) GetSubscriptionInfo(userId int) (SubscriptionInfo, error) {
	coll := ms.subscriptionsCollection
	filter := bson.D{{Key: "userId", Value: userId}}
	var res SubscriptionInfo
	err := coll.FindOne(context.TODO(), filter).Decode(&res)
	return res, err
}

func (ms *MongoStorage) UnsubscribeUser(categoryName string, userId int) (bool, error) {
	subInfo, err := ms.GetSubscriptionInfo(userId)
	if err != nil {
		return false, err
	}

	isFound := false
	for id, sub := range subInfo.Subscriptions {
		if sub.NameCategory == categoryName {
			isFound = true
			subInfo.Subscriptions = remove(subInfo.Subscriptions, id)
			break
		}
	}

	if !isFound {
		return false, nil
	}

	coll := ms.subscriptionsCollection
	filter := bson.D{{Key: "userId", Value: userId}}
	if _, err := coll.ReplaceOne(context.TODO(), filter, subInfo); err != nil {
		return false, err
	}

	return true, nil
}

func (ms *MongoStorage) SubscribeUser(category DouCategory, userId int, chatId int64, userName string) (bool, error) {
	coll := ms.subscriptionsCollection
	filter := bson.D{{Key: "userId", Value: userId}}
	subCategory := SubscriptionCategory{IDCategory: GetId(category.id), NameCategory: category.name}
	var res SubscriptionInfo
	coll.FindOne(context.TODO(), filter).Decode(&res)
	res.ChatId = chatId
	res.UserName = userName
	if res.UserId == 0 {
		fmt.Println("User doesn't exist, creating...")
		res.UserId = userId
		res.CreateDate = time.Now().UTC().Format(time.RFC1123Z)
		res.Subscriptions = []SubscriptionCategory{subCategory}
		if _, err := coll.InsertOne(context.TODO(), res); err != nil {
			return false, err
		}
		fmt.Printf("User with name %s created\n", userName)
		return true, nil
	}

	for _, alreadySubCat := range res.Subscriptions {
		if alreadySubCat.IDCategory == GetId(category.id) {
			return false, nil
		}
	}

	res.Subscriptions = append(res.Subscriptions, subCategory)
	if _, err := coll.ReplaceOne(context.TODO(), filter, res); err != nil {
		return false, err
	}

	return true, nil
}

func (ms *MongoStorage) SetLastTimeCheckedUTC(category DouCategory) error {
	coll := ms.categoriesCollection
	c := &CategoryInfo{
		IDCategory:      GetId(category.id),
		NameCategory:    category.name,
		LastTimeChecked: time.Now().UTC().Format(time.RFC1123Z),
	}

	filter := bson.D{{Key: "idCategory", Value: c.IDCategory}}
	result, err := coll.ReplaceOne(context.TODO(), filter, c)
	if err != nil {
		fmt.Println(err)
		return err
	}

	if result.MatchedCount == 0 {
		res, err := coll.InsertOne(context.TODO(), c)
		if err != nil {
			return err
		}
		fmt.Printf("Added category %v with id: %v\n", category.name, res.InsertedID)
		return nil
	}
	fmt.Printf("Replaced lastTimeUsed for category `%v` matches: %v\n", category.name, result.MatchedCount)

	return nil
}
func (ms *MongoStorage) GetLastTimeCheckedUTC(category DouCategory) time.Time {
	coll := ms.categoriesCollection
	filter := bson.D{{Key: "idCategory", Value: GetId(category.id)}}

	var doc CategoryInfo
	result := coll.FindOne(context.TODO(), filter)
	if err := result.Decode(&doc); err != nil {
		fmt.Printf("Category %s:id[%s] wasn't found, so using current time\n", category.name, category.id)
		return time.Now().UTC()
	}

	tm, err := time.Parse(time.RFC1123Z, fmt.Sprint(doc.LastTimeChecked))
	if err != nil {
		fmt.Printf("Error parsing %s to time\n", doc.LastTimeChecked)
		return time.Now().UTC()
	}

	return tm //time.Date(2023, time.March, 9, 18, 0, 0, 0, time.Now().Location()).UTC()
}

func remove[T any](slice []T, s int) []T {
	return append(slice[:s], slice[s+1:]...)
}

func GetId(id string) string {
	if id == "" {
		return "all"
	}
	return id
}
