package function

import (
	// "github.com/aws/aws-lambda-go/lambda"

	"handler/function/geo"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/eniac/Beldi/data"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	req := data.GeoRequest{}
	err := mapstructure.Decode(env.Input, &req)
	beldilib.CHECK(err)
	return geo.Nearby(env, req)
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	lambdacontext.FunctionName = "beldi-dev-geo"
	return beldilib.Wrapper(Handler)(req)
}
