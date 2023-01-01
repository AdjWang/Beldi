module handler/function

go 1.19

require (
	github.com/aws/aws-lambda-go v1.19.1
	github.com/aws/aws-sdk-go v1.34.6
	github.com/eniac/Beldi v0.0.0-20221111215415-80ec197583b6
	github.com/lithammer/shortuuid v3.0.0+incompatible
	github.com/mitchellh/mapstructure v1.5.0
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.opentelemetry.io/otel v1.11.2 // indirect
	go.opentelemetry.io/otel/exporters/jaeger v1.11.2 // indirect
	go.opentelemetry.io/otel/sdk v1.11.2 // indirect
	go.opentelemetry.io/otel/trace v1.11.2 // indirect
	golang.org/x/sys v0.0.0-20220919091848-fb04ddd9f9c8 // indirect
)

replace handler/function => ./

replace github.com/eniac/Beldi => ./
