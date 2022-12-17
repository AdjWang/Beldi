package function

import (
	"github.com/eniac/Beldi/data"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/eniac/Beldi/rate"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	req := data.RateRequest{}
	err := mapstructure.Decode(env.Input, &req)
	beldilib.CHECK(err)
	return rate.GetRates(env, req)
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	return beldilib.Wrapper(Handler)(req)
}
