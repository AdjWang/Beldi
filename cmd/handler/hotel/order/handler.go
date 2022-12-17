package function

import (
	"github.com/eniac/Beldi/data"
	"github.com/eniac/Beldi/order"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	var rpcInput data.RPCInput
	beldilib.CHECK(mapstructure.Decode(env.Input, &rpcInput))
	req := rpcInput.Input.(map[string]interface{})
	order.PlaceOrder(env, req["userId"].(string), req["flightId"].(string), req["hotelId"].(string))
	return 0
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	return beldilib.Wrapper(Handler)(req)
}
