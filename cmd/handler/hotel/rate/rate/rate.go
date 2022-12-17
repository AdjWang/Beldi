package rate

import (
	"sort"

	"github.com/eniac/Beldi/data"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/mitchellh/mapstructure"
)

func GetRates(env *beldilib.Env, req data.RateRequest) data.RateResult {
	var plans RatePlans
	for _, i := range req.HotelIds {
		plan := data.RatePlan{}
		res := beldilib.Read(env, data.Trate(), i)
		err := mapstructure.Decode(res, &plan)
		beldilib.CHECK(err)
		if plan.HotelId != "" {
			plans = append(plans, plan)
		}
	}
	sort.Sort(plans)
	return data.RateResult{RatePlans: plans}
}
