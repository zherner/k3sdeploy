package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"log"
)

// initAWS uses Default Config with specified region to authenticate
// using the SDK's default configuration, loading additional config
// and credentials values from the environment variables, shared
// credentials, and shared configuration files
func initAWS() aws.Config {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	return cfg
}

// getCallerId prints the ARN and userID of the request maker from cfg
func getCallerId(cfg aws.Config) {

	client := sts.NewFromConfig(cfg)

	result, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatalf("failed to get identity, %v", err)
	}

	fmt.Println(*result.Arn + " - " + *result.UserId)
}
