package profile

import "github.com/eniac/Beldi/data"

type Request struct {
	HotelIds []string
	Locale   string
}

type Result struct {
	Hotels []data.Hotel
}
