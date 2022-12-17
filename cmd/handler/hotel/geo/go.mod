module handler/function

go 1.19

require (
	github.com/aws/aws-lambda-go v1.19.1
	github.com/aws/aws-sdk-go v1.34.6
	github.com/eniac/Beldi v0.0.0-20221111215415-80ec197583b6
	github.com/hailocab/go-geoindex v0.0.0-20160127134810-64631bfe9711
	github.com/lithammer/shortuuid v3.0.0+incompatible
	github.com/mitchellh/mapstructure v1.5.0
)

require (
	github.com/google/uuid v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.3.0 // indirect
)

replace handler/function => ./

replace github.com/eniac/Beldi => ./
