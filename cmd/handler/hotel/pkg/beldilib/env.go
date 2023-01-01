package beldilib

import (
	"context"
)

type Env struct {
	Ctx context.Context

	LambdaId    string
	InstanceId  string
	LogTable    string
	IntentTable string
	LocalTable  string
	StepNumber  int32
	Input       interface{}
	TxnId       string
	Instruction string
	Baseline    bool
}
