package function

import (
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/eniac/Beldi/data"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/eniac/Beldi/search"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	req := data.SearchRequest{}
	err := mapstructure.Decode(env.Input, &req)
	beldilib.CHECK(err)
	return aws.JSONValue{"search": search.Nearby(env, req)}
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	lambdacontext.FunctionName = "beldi-dev-search"
	return beldilib.Wrapper(Handler)(req)
}
