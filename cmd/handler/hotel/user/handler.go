package function

import (
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/eniac/Beldi/user"
	"github.com/mitchellh/mapstructure"
)

func Handler(env *beldilib.Env) interface{} {
	req := user.Request{}
	err := mapstructure.Decode(env.Input, &req)
	beldilib.CHECK(err)
	return user.CheckUser(env, req)
}

// func main() {
// 	lambda.Start(beldilib.Wrapper(Handler))
// }

func Handle(req []byte) string {
	return beldilib.Wrapper(Handler)(req)
}
