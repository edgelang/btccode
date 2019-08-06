package main

// import (
// 	"context"
// 	"os"
// 	"time"

// 	log "github.com/inconshreveable/log15"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// var (
// 	watchDB  *mongo.Collection
// 	mongoURL = "mongodb://127.0.0.1:27017"
// )

// func init() {
// 	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
// 	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
// 	if err != nil {
// 		log.Crit("can not connect mongodb")
// 		os.Exit(1)
// 	}
// 	watchDB = client.Database("btcusdt").Collection("watch")
// }

// type watchSchema struct {
// 	Txid          string
// 	Confirmations int
// 	Symbol        string
// 	From          string
// 	To            string
// 	Amount        string
// }

// func hasTxid(txid string) bool {
// 	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
// 	if err := watchDB.FindOne(ctx, bson.M{"txid": txid}).Err(); err != nil {
// 		return false
// 	}
// 	return true
// }

// func addTx(tx *watchSchema) {

// }

// func updateTx(tx *watchSchema) {
// 	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
// 	if hasTxid(tx.Txid) {
// 		watchDB.UpdateOne(ctx, bson.M{"txid": tx.Txid}, tx)
// 	}
// 	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
// }
