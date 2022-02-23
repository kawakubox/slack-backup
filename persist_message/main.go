package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"
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
	message  *events.SQSMessage
	s3Event  *events.S3Event
	s3Bucket *events.S3Bucket
	s3Object *events.S3Object
}

func NewMessagePersistenceService(message *events.SQSMessage) (*MessagePersistenceService, error) {
	var event events.S3Event
	err := json.Unmarshal([]byte(message.Body), &event)
	if err != nil {
		log.Fatal().Err(err)
		return nil, err
	}

	var instance MessagePersistenceService
	instance.message = message
	instance.s3Bucket = &event.Records[0].S3.Bucket
	instance.s3Object = &event.Records[0].S3.Object

	return &instance, nil
}

func (svc *MessagePersistenceService) BucketName() string {
	return svc.s3Bucket.Name
}

func (svc *MessagePersistenceService) ObjectKey() string {
	return svc.s3Object.Key
}

func (svc *MessagePersistenceService) ChannelName() string {
	separated := strings.Split(svc.s3Object.Key, "/")
	return separated[len(separated)-2]
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

func (svc *MessagePersistenceService) persist(ctx context.Context, cfg aws.Config, messages []Message) error {
	client := dynamodb.NewFromConfig(cfg)

	for _, msg := range messages {
		putItemInput := &dynamodb.PutItemInput{
			TableName: aws.String("splathon-slack-message"),
			Item: map[string]types.AttributeValue{
				"ClientMessageID": &types.AttributeValueMemberS{Value: msg.ClientMessageID},
				"Channel":         &types.AttributeValueMemberS{Value: svc.ChannelName()},
				"User":            &types.AttributeValueMemberS{Value: msg.User},
				"Text":            &types.AttributeValueMemberS{Value: msg.Text},
				"Timestamp":       &types.AttributeValueMemberN{Value: msg.UnixTimestamp},
			},
		}

		log.Debug().Msgf("%+v", putItemInput)

		_, err := client.PutItem(ctx, putItemInput)
		if err != nil {
			log.Fatal().Err(err)
			return err
		}
	}
	return nil
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

	// Persist Slack message
	log.Info().Msg("[START] Persist Slack message")

	svc.persist(ctx, cfg, messages)

	log.Info().Msg("[END] Persist Slack message")

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
