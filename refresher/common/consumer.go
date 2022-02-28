package common

import (
	"fmt"
	"github.com/infralight/go-kit/db/mongo"
	"github.com/infralight/pulumi/refresher/config"
)

type Consumer struct {
	MongoDb *mongo.Client
	Config *config.Config
}

func NewConsumer(cfg *config.Config) (*Consumer, error) {
	var err error

	consumer := &Consumer{
		Config:  cfg,
	}

	consumer.MongoDb, err = mongo.NewClient(cfg.MongoURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongoDB: %v", err)
	}

	return consumer, nil

}