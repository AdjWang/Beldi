package function

import (
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/eniac/Beldi/profile"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	req := profile.Request{}
	err := mapstructure.Decode(env.Input, &req)
	beldilib.CHECK(err)
	return profile.GetProfiles(env, req)
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	lambdacontext.FunctionName = "beldi-dev-profile"
	return beldilib.Wrapper(Handler)(req)
}
