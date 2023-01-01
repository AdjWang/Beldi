package function

import (
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/eniac/Beldi/pkg/beldilib"
)

func Handler(env *beldilib.Env) interface{} {
	beldilib.RestartAll("gateway")
	return "ok"
}

// func main() {
// 	lambda.Start(Handler)
// }

func Handle(req []byte) string {
	lambdacontext.FunctionName = "beldi-dev-hotelcollector"
	return beldilib.Wrapper(Handler)(req)
}
