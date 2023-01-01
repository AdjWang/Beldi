package beldilib

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	trace "github.com/eniac/Beldi/pkg/trace"
	"github.com/lithammer/shortuuid"
	"github.com/mitchellh/mapstructure"
)

type InputWrapper struct {
	CallerName  string      `mapstructure:"CallerName"`
	CallerId    string      `mapstructure:"CallerId"`
	CallerStep  int32       `mapstructure:"CallerStep"`
	InstanceId  string      `mapstructure:"InstanceId"`
	Input       interface{} `mapstructure:"Input"`
	TxnId       string      `mapstructure:"TxnId"`
	Instruction string      `mapstructure:"Instruction"`
	Async       bool        `mapstructure:"Async"`

	// propagate the ctx of parent function to relate trace info together
	TraceContextCarrier map[string]string `mapstructure:"TraceContextCarrier"`
}

func (iw *InputWrapper) Serialize() []byte {
	stream, err := json.Marshal(*iw)
	CHECK(err)
	return stream
}

func (iw *InputWrapper) Deserialize(stream []byte) {
	err := json.Unmarshal(stream, iw)
	CHECK(err)
}

type StackTraceCall struct {
	Label string `json:"label"`
	Line  int    `json:"line"`
	Path  string `json:"path"`
}

func (ie *InvokeError) Deserialize(stream []byte) {
	err := json.Unmarshal(stream, ie)
	CHECK(err)
	if ie.ErrorMessage == "" {
		panic(errors.New("never happen"))
	}
}

type InvokeError struct {
	ErrorMessage string           `json:"errorMessage"`
	ErrorType    string           `json:"errorType"`
	StackTrace   []StackTraceCall `json:"stackTrace"`
}

type OutputWrapper struct {
	Status string
	Output interface{}
}

func (ow *OutputWrapper) Serialize() []byte {
	stream, err := json.Marshal(*ow)
	CHECK(err)
	return stream
}

func (ow *OutputWrapper) Deserialize(stream []byte) {
	err := json.Unmarshal(stream, ow)
	CHECK(errors.Wrapf(err, "json unmarshal err: %v", string(stream)))
	if ow.Status != "Success" && ow.Status != "Failure" {
		ie := InvokeError{}
		ie.Deserialize(stream)
		panic(ie)
	}
}

//	func ParseInput(raw interface{}) *InputWrapper {
//		var iw InputWrapper
//		if body, ok := raw.(map[string]interface{})["body"]; ok {
//			CHECK(json.Unmarshal([]byte(body.(string)), &iw))
//		} else {
//			CHECK(mapstructure.Decode(raw, &iw))
//		}
//		return &iw
//	}
//

// Adjust for OpenFaaS
func ParseInput(raw []byte) *InputWrapper {
	var iw InputWrapper
	CHECK(json.Unmarshal(raw, &iw))
	return &iw
}

func PrepareEnv(iw *InputWrapper) *Env {
	s := strings.Split(lambdacontext.FunctionName, "-")
	lambdaId := s[len(s)-1]
	if iw.InstanceId == "" {
		iw.InstanceId = shortuuid.New()
	}
	ctx := context.Background()
	if iw.CallerName != "" { // current func is not the root
		ctx = trace.ExtractTraceContextFromCarrier(iw.TraceContextCarrier)
	}

	return &Env{
		Ctx: ctx,

		LambdaId:    lambdaId,
		InstanceId:  iw.InstanceId,
		LogTable:    fmt.Sprintf("%s-log", lambdaId),
		IntentTable: fmt.Sprintf("%s-collector", lambdaId),
		LocalTable:  fmt.Sprintf("%s-local", lambdaId),
		StepNumber:  0,
		Input:       iw.Input,
		TxnId:       iw.TxnId,
		Instruction: iw.Instruction,
	}
}

func SyncInvoke(env *Env, callee string, input interface{}) (interface{}, string) {
	originalCtx := env.Ctx
	ctx, funcSpan := trace.NewSpan(env.Ctx, fmt.Sprintf("SyncInvoke callee: %v", callee))
	env.Ctx = ctx
	defer func() {
		funcSpan.End()
		env.Ctx = originalCtx
	}()

	if TYPE == "BASELINE" {
		iw := InputWrapper{
			InstanceId:  "",
			Input:       input,
			CallerName:  "",
			Async:       false,
			TxnId:       env.TxnId,
			Instruction: env.Instruction,

			TraceContextCarrier: trace.MakeTraceContextToCarrier(env.Ctx), // for tracer
		}
		if iw.Instruction == "EXECUTE" {
			LibWrite(env.LocalTable, aws.JSONValue{"K": env.TxnId}, map[expression.NameBuilder]expression.OperandBuilder{
				expression.Name("CALLEES"): expression.Name("CALLEES").ListAppend(expression.Value([]string{callee})),
			})
		}
		payload := iw.Serialize()
		// res, err := LambdaClient.Invoke(&lambdaSdk.InvokeInput{
		// 	FunctionName: aws.String(fmt.Sprintf("beldi-dev-%s", callee)),
		// 	Payload:      payload,
		// })
		res, err := OpenFaaSSyncInvoke(fmt.Sprintf("beldi-dev-%s", callee), payload)
		CHECK(err)
		ow := OutputWrapper{}
		// ow.Deserialize(res.Payload)
		ow.Deserialize([]byte(res))
		switch ow.Status {
		case "Success":
			return ow.Output, iw.InstanceId
		default:
			panic("never happens")
		}
	}
	iw := InputWrapper{
		CallerName:  env.LambdaId,
		CallerId:    env.InstanceId,
		CallerStep:  env.StepNumber,
		Async:       false,
		InstanceId:  shortuuid.New(),
		Input:       input,
		TxnId:       env.TxnId,
		Instruction: env.Instruction,

		TraceContextCarrier: trace.MakeTraceContextToCarrier(env.Ctx), // for tracer
	}
	pk := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}
	_, subSpan1 := trace.NewSpan(env.Ctx, fmt.Sprintf("LibPut pk: %v, callee: %v %v", pk, iw.InstanceId, callee))
	ok := LibPut(env.LogTable, pk, aws.JSONValue{"Callee": iw.InstanceId})
	subSpan1.End()
	if !ok {
		_, subSpan2 := trace.NewSpan(env.Ctx, fmt.Sprintf("LibRead last result pk: %v", pk))
		item := LibRead(env.LogTable, pk, []string{"Callee", "RET"})
		subSpan2.End()
		if val, exist := item["Callee"].(string); exist {
			iw.InstanceId = val
		} else {
			panic("error")
		}
		if val, exist := item["RET"]; exist {
			env.StepNumber += 1
			return val, iw.InstanceId
		}
	}
	env.StepNumber += 1
	if iw.Instruction == "EXECUTE" {
		EOSWrite(env, env.LocalTable, env.TxnId, map[expression.NameBuilder]expression.OperandBuilder{
			expression.Name("CALLEES"): expression.Name("CALLEES").ListAppend(expression.Value([]string{callee})),
		})
	}
	payload := iw.Serialize()
	// res, err := LambdaClient.Invoke(&lambdaSdk.InvokeInput{
	// 	FunctionName: aws.String(fmt.Sprintf("beldi-dev-%s", callee)),
	// 	Payload:      payload,
	// })
	_, subSpan3 := trace.NewSpan(env.Ctx, fmt.Sprintf("Gateway SyncInvoke callee: %v", callee))
	res, err := OpenFaaSSyncInvoke(fmt.Sprintf("beldi-dev-%s", callee), payload)
	subSpan3.End()
	CHECK(err)
	ow := OutputWrapper{}
	// ow.Deserialize(res.Payload)
	ow.Deserialize([]byte(res))
	switch ow.Status {
	case "Success":
		return ow.Output, iw.InstanceId
	default:
		panic("never happens")
	}
}

func AssignedSyncInvoke(env *Env, callee string, input interface{}, stepNumber int32) (interface{}, string) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("AssignedSyncInvoke callee: %v", callee))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	if TYPE == "BASELINE" {
		return SyncInvoke(env, callee, input)
	}
	iw := InputWrapper{
		CallerName:  env.LambdaId,
		CallerId:    env.InstanceId,
		CallerStep:  stepNumber,
		Async:       false,
		InstanceId:  shortuuid.New(),
		Input:       input,
		TxnId:       env.TxnId,
		Instruction: env.Instruction,

		TraceContextCarrier: trace.MakeTraceContextToCarrier(env.Ctx), // for tracer
	}
	pk := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": stepNumber}
	_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("LibPut pk: %v, callee: %v", pk, iw.InstanceId))
	ok := LibPut(env.LogTable, pk, aws.JSONValue{"Callee": iw.InstanceId})
	span.End()
	if !ok {
		_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("LibRead last result pk: %v", pk))
		item := LibRead(env.LogTable, pk, []string{"Callee", "RET"})
		span.End()
		if val, exist := item["Callee"].(string); exist {
			iw.InstanceId = val
		} else {
			panic("error")
		}
		if val, exist := item["RET"]; exist {
			return val, iw.InstanceId
		}
	}
	if iw.Instruction == "EXECUTE" {
		EOSWrite(env, env.LocalTable, env.TxnId, map[expression.NameBuilder]expression.OperandBuilder{
			expression.Name("CALLEES"): expression.Name("CALLEES").ListAppend(expression.Value([]string{callee})),
		})
	}
	payload := iw.Serialize()
	// res, err := LambdaClient.Invoke(&lambdaSdk.InvokeInput{
	// 	FunctionName: aws.String(fmt.Sprintf("beldi-dev-%s", callee)),
	// 	Payload:      payload,
	// })
	_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("Gateway SyncInvoke callee: %v", callee))
	res, err := OpenFaaSSyncInvoke(fmt.Sprintf("beldi-dev-%s", callee), payload)
	span.End()
	CHECK(err)
	ow := OutputWrapper{}
	// ow.Deserialize(res.Payload)
	ow.Deserialize([]byte(res))
	switch ow.Status {
	case "Success":
		return ow.Output, iw.InstanceId
	default:
		panic("never happens")
	}
}

func AsyncInvoke(env *Env, callee string, input interface{}) string {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("AsyncInvoke callee: %v", callee))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	if TYPE == "BASELINE" {
		iw := InputWrapper{
			InstanceId: "",
			Async:      true,
			CallerName: "",
			Input:      input,

			TraceContextCarrier: trace.MakeTraceContextToCarrier(env.Ctx), // for tracer
		}
		payload := iw.Serialize()
		// _, err := LambdaClient.Invoke(&lambdaSdk.InvokeInput{
		// 	FunctionName:   aws.String(fmt.Sprintf("beldi-dev-%s", callee)),
		// 	Payload:        payload,
		// 	InvocationType: aws.String("Event"),
		// })
		err := OpenFaaSAsyncInvoke(fmt.Sprintf("beldi-dev-%s", callee), payload)
		CHECK(err)
		return ""
	}

	iw := InputWrapper{
		CallerName: env.LambdaId,
		CallerId:   env.InstanceId,
		CallerStep: env.StepNumber,
		Async:      true,
		InstanceId: shortuuid.New(),
		Input:      input,

		TraceContextCarrier: trace.MakeTraceContextToCarrier(env.Ctx), // for tracer
	}

	pk := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}
	_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("LibPut pk: %v, callee: %v", pk, iw.InstanceId))
	ok := LibPut(env.LogTable, pk, aws.JSONValue{"Callee": iw.InstanceId})
	span.End()
	if !ok {
		_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("LibRead last result pk: %v", pk))
		item := LibRead(env.LogTable, pk, []string{"Callee", "RET"})
		span.End()
		if val, exist := item["Callee"].(string); exist {
			iw.InstanceId = val
		} else {
			panic("error")
		}
		if _, exist := item["RET"]; exist {
			env.StepNumber += 1
			return iw.InstanceId
		}
	}

	_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("LibPut callee: %v", iw.InstanceId))
	ok = LibPut(fmt.Sprintf("%s-collector", callee), aws.JSONValue{"InstanceId": iw.InstanceId},
		aws.JSONValue{"DONE": false, "ASYNC": true, "INPUT": iw.Input, "ST": time.Now().Unix()})
	span.End()

	if !ok {
		env.StepNumber += 1
		return iw.InstanceId
	}

	_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("LibWrite pk: %v", pk))
	LibWrite(env.LogTable, pk, map[expression.NameBuilder]expression.OperandBuilder{
		expression.Name("RET"): expression.Value(1),
	})
	span.End()

	payload := iw.Serialize()
	// _, err := LambdaClient.Invoke(&lambdaSdk.InvokeInput{
	// 	FunctionName:   aws.String(fmt.Sprintf("beldi-dev-%s", callee)),
	// 	Payload:        payload,
	// 	InvocationType: aws.String("Event"),
	// })
	_, span = trace.NewSpan(env.Ctx, fmt.Sprintf("Gateway AsyncInvoke callee: %v", callee))
	err := OpenFaaSAsyncInvoke(fmt.Sprintf("beldi-dev-%s", callee), payload)
	span.End()
	CHECK(err)
	env.StepNumber += 1
	return iw.InstanceId
}

func TPLCommit(env *Env) {
	ctx, span := trace.NewSpan(env.Ctx, "TPLCommit")
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	item := EOSRead(env, env.LocalTable, env.TxnId, []string{})
	var callees []string
	for k, v := range item {
		if k == "CALLEES" {
			CHECK(mapstructure.Decode(v, &callees))
			continue
		}
		ks := strings.Split(k, "-")
		if len(ks) != 2 {
			continue
		}
		tablename, key := ks[0], ks[1]
		update := map[expression.NameBuilder]expression.OperandBuilder{}
		for kk, vv := range v.(map[string]interface{}) {
			update[expression.Name(kk)] = expression.Value(vv)
		}
		update[expression.Name("HOLDER")] = expression.Value(AVAILABLE)
		EOSWrite(env, tablename, key, update)
	}
	LibDelete(env.LocalTable, aws.JSONValue{"K": env.TxnId, "ROWHASH": "HEAD"})
	for _, callee := range callees {
		if callee == " " {
			continue
		}
		SyncInvoke(env, callee, aws.JSONValue{})
	}
}

func TPLAbort(env *Env) {
	ctx, span := trace.NewSpan(env.Ctx, "TPLAbort")
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	item := EOSRead(env, env.LocalTable, env.TxnId, []string{"CALLEES"})
	var callees []string
	for k, v := range item {
		if k == "CALLEES" {
			CHECK(mapstructure.Decode(v, &callees))
			continue
		}
		ks := strings.Split(k, "-")
		if len(ks) != 2 {
			continue
		}
		tablename, key := ks[0], ks[1]
		update := map[expression.NameBuilder]expression.OperandBuilder{}
		update[expression.Name("HOLDER")] = expression.Value(AVAILABLE)
		EOSWrite(env, tablename, key, update)
	}
	LibDelete(env.LocalTable, aws.JSONValue{"K": env.TxnId, "ROWHASH": "HEAD"})
	for _, callee := range callees {
		if callee == " " {
			continue
		}
		SyncInvoke(env, callee, aws.JSONValue{})
	}
}

//	func Wrapper(f func(env *Env) interface{}) func(iw interface{}) (OutputWrapper, error) {
//		return func(raw interface{}) (OutputWrapper, error) {
func Wrapper(f func(env *Env) interface{}) func(iw []byte) string {
	return func(raw []byte) string {
		iw := ParseInput(raw)
		env := PrepareEnv(iw)

		// Bootstrap tracer.
		prv, err := trace.NewProvider(trace.ProviderConfig{
			JaegerEndpoint: TraceReceiverEndpoint,
			ServiceName:    lambdacontext.FunctionName,
			ServiceVersion: TraceReceiverServiceVersion,
			Environment:    TraceReceiverEnvironment,
			Disabled:       false,
		})
		if err != nil {
			panic(err)
		}
		defer prv.Close(env.Ctx)

		originalCtx := env.Ctx
		ctx, span := trace.NewSpan(env.Ctx, "Wrapper")
		env.Ctx = ctx
		defer func() {
			span.End()
			env.Ctx = originalCtx
		}()

		if TYPE != "BASELINE" {
			if iw.Async == false || iw.CallerName == "" {
				_, span := trace.NewSpan(ctx, fmt.Sprintf("pre LibPut IntentTable: %v, key: %v", env.IntentTable, env.InstanceId))
				LibPut(env.IntentTable, aws.JSONValue{"InstanceId": env.InstanceId},
					aws.JSONValue{"DONE": false, "ASYNC": iw.Async, "INPUT": iw.Input, "ST": time.Now().Unix()})
				span.End()
			} else {
				_, span := trace.NewSpan(ctx, fmt.Sprintf("pre LibWrite IntentTable: %v, key: %v", env.IntentTable, env.InstanceId))
				LibWrite(env.IntentTable, aws.JSONValue{"InstanceId": env.InstanceId},
					map[expression.NameBuilder]expression.OperandBuilder{
						expression.Name("ST"): expression.Value(time.Now().Unix()),
					})
				span.End()
			}
			//ok := LibPut(env.IntentTable, aws.JSONValue{"InstanceId": env.InstanceId},
			//	aws.JSONValue{"DONE": false, "ASYNC": iw.Async})
			//if !ok {
			//	res := LibRead(env.IntentTable, aws.JSONValue{"InstanceId": env.InstanceId}, []string{"RET"})
			//	output, exist := res["RET"]
			//	if exist {
			//		return OutputWrapper{
			//			Status: "Success",
			//			Output: output,
			//		}, nil
			//	}
			//}
		}

		var output interface{}
		if env.Instruction == "COMMIT" {
			TPLCommit(env)
			output = 0
		} else if env.Instruction == "ABORT" {
			TPLAbort(env)
			output = 0
		} else if env.Instruction == "EXECUTE" {
			EOSWrite(env, env.LocalTable, env.TxnId, map[expression.NameBuilder]expression.OperandBuilder{
				expression.Name("CALLEES"): expression.Value([]string{" "}),
			})
			originalCtx := ctx
			subCtx, span := trace.NewSpan(ctx, "user func")
			env.Ctx = subCtx
			output = f(env)
			span.End()
			env.Ctx = originalCtx
		} else {
			originalCtx := ctx
			subCtx, span := trace.NewSpan(ctx, "user func")
			env.Ctx = subCtx
			output = f(env)
			span.End()
			env.Ctx = originalCtx
		}

		if TYPE != "BASELINE" {
			if iw.CallerName != "" {
				_, span := trace.NewSpan(ctx, fmt.Sprintf("pre LibWrite (callback) caller: %v, id: %v, step: %v", iw.CallerName, iw.CallerId, iw.CallerStep))
				LibWrite(fmt.Sprintf("%s-log", iw.CallerName),
					aws.JSONValue{"InstanceId": iw.CallerId, "StepNumber": iw.CallerStep},
					map[expression.NameBuilder]expression.OperandBuilder{
						expression.Name("RET"): expression.Value(output),
					})
				span.End()
			}
			_, span := trace.NewSpan(ctx, fmt.Sprintf("pre LibWrite (done) IntentTable: %v, key: %v", env.IntentTable, env.InstanceId))
			LibWrite(env.IntentTable, aws.JSONValue{"InstanceId": env.InstanceId},
				map[expression.NameBuilder]expression.OperandBuilder{
					expression.Name("DONE"): expression.Value(true),
					expression.Name("TS"):   expression.Value(time.Now().Unix()),
					//expression.Name("RET"):  expression.Value(output),
				})
			span.End()
		}
		// return OutputWrapper{
		// 	Status: "Success",
		// 	Output: output,
		// }
		ret := OutputWrapper{
			Status: "Success",
			Output: output,
		}
		return string(ret.Serialize())
	}
}
