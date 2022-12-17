package function

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/eniac/Beldi/recommendation"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	req := recommendation.Request{}
	err := mapstructure.Decode(env.Input, &req)
	beldilib.CHECK(err)
	res := recommendation.GetRecommendations(env, req)
	return aws.JSONValue{"recommend": res}
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	return beldilib.Wrapper(Handler)(req)
}
