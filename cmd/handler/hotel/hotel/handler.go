package function

import (
	"github.com/eniac/Beldi/data"
	"github.com/eniac/Beldi/hotel"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	var rpcInput data.RPCInput
	beldilib.CHECK(mapstructure.Decode(env.Input, &rpcInput))
	req := rpcInput.Input.(map[string]interface{})
	switch rpcInput.Function {
	case "ReserveHotel":
		return hotel.ReserveHotel(env, req["hotelId"].(string), req["userId"].(string))
	case "BaseReserveHotel":
		return hotel.BaseReserveHotel(env, req["hotelId"].(string), req["userId"].(string))
	case "AddHotel":
		hotel.AddHotel(env, req["hotelId"].(string), int32(req["cap"].(float64)))
		return 0
	}
	panic("no such function")
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	return beldilib.Wrapper(Handler)(req)
}
