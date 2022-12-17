package search

import (
	"github.com/eniac/Beldi/data"
	"github.com/eniac/Beldi/pkg/beldilib"
	"github.com/mitchellh/mapstructure"
)

func Nearby(env *beldilib.Env, req data.SearchRequest) data.SearchResult {
	res, _ := beldilib.SyncInvoke(env, data.Tgeo(), data.GeoRequest{Lat: req.Lat, Lon: req.Lon})
	var geoRes data.GeoResult
	beldilib.CHECK(mapstructure.Decode(res, &geoRes))
	res, _ = beldilib.SyncInvoke(env, data.Trate(), data.RateRequest{
		HotelIds: geoRes.HotelIds,
		Indate:   req.InDate,
		Outdate:  req.OutDate,
	})
	var rateRes data.RateResult
	beldilib.CHECK(mapstructure.Decode(res, &rateRes))
	var hts []string
	for _, r := range rateRes.RatePlans {
		hts = append(hts, r.HotelId)
	}
	return data.SearchResult{HotelIds: hts}
}
