package function

import (
	"github.com/eniac/Beldi/frontend"
	"github.com/eniac/Beldi/pkg/beldilib"
)

func Handler(env *beldilib.Env) interface{} {
	req := env.Input.(map[string]interface{})
	return frontend.SendRequest(env, req["userId"].(string), req["flightId"].(string), req["hotelId"].(string))
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	return beldilib.Wrapper(Handler)(req)
}
