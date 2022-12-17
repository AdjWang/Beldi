module handler/function

go 1.19

require github.com/eniac/Beldi v0.0.0-20221111215415-80ec197583b6

require (
	github.com/aws/aws-lambda-go v1.19.1 // indirect
	github.com/aws/aws-sdk-go v1.34.6 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.3.0 // indirect
	github.com/lithammer/shortuuid v3.0.0+incompatible // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
)

replace handler/function => ./

replace github.com/eniac/Beldi => ./
