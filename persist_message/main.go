package main

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rs/zerolog/log"
	"net/http"
)

type Response events.APIGatewayProxyResponse

func Handler(ctx context.Context, event events.SQSEvent) (Response, error) {
	json, err := json.Marshal(event)
	if err != nil {
		log.Fatal().Err(err)
		res := buildResponse(http.StatusInternalServerError, "Parsing SQS Event is failed.")
		return res, err
	}
	log.Info().RawJSON("event", json).Msg("")

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
