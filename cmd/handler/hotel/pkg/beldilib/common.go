package beldilib

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/lambda"
)

var sess = session.Must(session.NewSessionWithOptions(session.Options{
	SharedConfigState: session.SharedConfigEnable,
}))

var LambdaClient = lambda.New(sess)

//var url = "http://133.130.115.39:8000"
//var DBClient = dynamodb.New(sess, &aws.Config{Endpoint: aws.String(url),
//	Region:                        aws.String("us-east-1"),
//	CredentialsChainVerboseErrors: aws.Bool(true)})

// var DBClient = dynamodb.New(sess)

var DBClient = dynamodb.New(sess,
	&aws.Config{
		Endpoint: aws.String("http://10.0.2.15:8000"),
		Region:   aws.String("us-east-1"),
		// Credentials:                   credentials.NewStaticCredentials("AKID", "SECRET_KEY", "TOKEN"),
		Credentials:                   credentials.NewStaticCredentials("2333", "abcd", "TOKEN"),
		CredentialsChainVerboseErrors: aws.Bool(true),
	})

var DLOGSIZE = "1000"

func GLOGSIZE() int {
	r, _ := strconv.Atoi(DLOGSIZE)
	return r
}

var T = int64(60)

var TYPE = "BELDI"

func CHECK(err error) {
	if err != nil {
		panic(err)
	}
}
