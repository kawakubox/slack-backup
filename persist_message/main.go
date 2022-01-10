package main

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"net/http"
)

type Response events.APIGatewayProxyResponse

func Handler(ctx context.Context) (Response, error) {
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
