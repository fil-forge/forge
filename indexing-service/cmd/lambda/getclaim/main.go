package main

import (
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/fil-forge/indexing-service/cmd/lambda"
	"github.com/fil-forge/indexing-service/pkg/aws"
	"github.com/fil-forge/indexing-service/pkg/server"
)

func main() {
	lambda.Start(makeHandler)
}

func makeHandler(cfg aws.Config) any {
	service, err := aws.Construct(cfg)
	if err != nil {
		panic(err)
	}

	handler := httpadapter.NewV2(server.GetClaimHandler(service)).ProxyWithContext

	return handler
}
