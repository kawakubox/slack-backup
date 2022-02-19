package main

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

type Response events.APIGatewayProxyResponse

type Message struct {
	Channel         string
	ClientMessageID string `json:"client_msg_id"`
	Type            string `json:"type"`
	Text            string `json:"text"`
	User            string `json:"user"`
	UnixTimestamp   string `json:"ts"`
}

type MessagePersistenceService struct {
	record   *events.SQSMessage
	s3Event  *events.S3Event
	s3Bucket *events.S3Bucket
	s3Object *events.S3Object
}

func NewMessagePersistenceService(message *events.SQSMessage) (*MessagePersistenceService, error) {
	var instance MessagePersistenceService
	instance.record = message

	body := []byte(message.Body)
	err := json.Unmarshal(body, instance.s3Event)
	if err != nil {
		log.Fatal().Err(err)
		return nil, err
	}
	log.Debug().Msgf("%v", instance.s3Event)

	instance.s3Bucket = &instance.s3Event.Records[0].S3.Bucket
	instance.s3Object = &instance.s3Event.Records[0].S3.Object

	return &instance, nil
}

func (svc *MessagePersistenceService) BucketName() string {
	return svc.s3Bucket.Name
}

func (svc *MessagePersistenceService) ObjectKey() string {
	return svc.s3Object.Key
}

func (svc *MessagePersistenceService) download(config aws.Config) (*s3.GetObjectOutput, error) {
	client := s3.NewFromConfig(config)

	return client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(svc.BucketName()),
		Key:    aws.String(svc.ObjectKey()),
	})
}

func (svc *MessagePersistenceService) parse(output *s3.GetObjectOutput) ([]Message, error) {
	buf, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, err
	}

	var messages []Message
	json.Unmarshal(buf, &messages)

	return messages, nil
}

func (svc *MessagePersistenceService) Persist(ctx context.Context) error {
	// Get a S3Object
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal().Err(err)
	}

	output, err := svc.download(cfg)
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msgf("%v", output)

	// Parse a json file
	messages, err := svc.parse(output)
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Debug().Msgf("%v", messages)

	return nil
}

func Handler(ctx context.Context, event events.SQSEvent) (Response, error) {
	json, err := json.Marshal(event)
	if err != nil {
		log.Fatal().Err(err)
		res := buildResponse(http.StatusInternalServerError, "Parsing SQS Event is failed.")
		return res, err
	}
	log.Info().RawJSON("event", json).Msg("")

	for _, record := range event.Records {
		log.Debug().Msgf("%v", record)
		svc := NewMessagePersistenceService(&record)
		svc.Persist(ctx)
	}

	res := buildResponse(http.StatusOK, "OK")
	return res, nil
}

func buildResponse(statusCode int, body string) Response {
	res := Response{
		StatusCode:      statusCode,
		IsBase64Encoded: false,
		Body:            body,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
	return res
}

func main() {
	lambda.Start(Handler)
}
