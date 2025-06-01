package main

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func getCalendar(calendarName *string) (*ssm.DescribeDocumentOutput, error) {
	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	// Create an Amazon S3 service client
	client := ssm.NewFromConfig(cfg)

	input := &ssm.DescribeDocumentInput{
		Name: aws.String(*calendarName),
	}

	// Get the first page of results for ListObjectsV2 for a bucket
	output, err := client.DescribeDocument(context.TODO(), input)
	if err != nil {
		log.Fatal(err)
	}

	return output, nil
}
