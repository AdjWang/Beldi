package beldilib

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	trace "github.com/eniac/Beldi/pkg/trace"
)

var AVAILABLE = "AVAILABLE"

func Lock(env *Env, tablename string, key string) bool {
	_, span := trace.NewSpan(env.Ctx, "Lock")
	defer span.End()

	cond1 := expression.AttributeNotExists(expression.Name("HOLDER"))
	cond2 := expression.Name("HOLDER").Equal(expression.Value(AVAILABLE))
	cond3 := expression.Name("HOLDER").Equal(expression.Value(env.TxnId))
	return EOSCondWrite(env, tablename, key, map[expression.NameBuilder]expression.OperandBuilder{
		expression.Name("HOLDER"): expression.Value(env.TxnId),
	}, cond1.Or(cond2.Or(cond3)))
}

func Unlock(env *Env, tablename string, key string) {
	_, span := trace.NewSpan(env.Ctx, "Unlock")
	defer span.End()

	cond := expression.Name("HOLDER").Equal(expression.Value(env.TxnId))
	EOSCondWrite(env, tablename, key, map[expression.NameBuilder]expression.OperandBuilder{
		expression.Name("HOLDER"): expression.Value(AVAILABLE),
	}, cond)
}

func TPLRead(env *Env, tablename string, key string, projection []string) (bool, aws.JSONValue) {
	_, span := trace.NewSpan(env.Ctx, "TPLRead")
	defer span.End()

	if Lock(env, tablename, key) {
		return true, EOSRead(env, tablename, key, projection)
	} else {
		return false, nil
	}
}

func TPLWrite(env *Env, tablename string, key string, value aws.JSONValue) bool {
	_, span := trace.NewSpan(env.Ctx, "TPLWrite")
	defer span.End()

	if Lock(env, tablename, key) {
		update := map[expression.NameBuilder]expression.OperandBuilder{}
		tablekey := fmt.Sprintf("%s-%s", tablename, key)
		update[expression.Name(tablekey)] = expression.Value(value)
		EOSWrite(env, env.LocalTable, env.TxnId, update)
		return true
	} else {
		return false
	}
}

func BeginTxn(env *Env) {
	_, span := trace.NewSpan(env.Ctx, "BeginTxn")
	defer span.End()

	env.TxnId = env.InstanceId
	EOSWrite(env, env.LocalTable, env.TxnId, map[expression.NameBuilder]expression.OperandBuilder{
		expression.Name("CALLEES"): expression.Value([]string{" "}),
	})
	env.Instruction = "EXECUTE"
}

func CommitTxn(env *Env) {
	_, span := trace.NewSpan(env.Ctx, "CommitTxn")
	defer span.End()

	env.Instruction = "COMMIT"
	TPLCommit(env)
	env.TxnId = ""
	env.Instruction = ""
}

func AbortTxn(env *Env) {
	_, span := trace.NewSpan(env.Ctx, "AbortTxn")
	defer span.End()

	env.Instruction = "ABORT"
	TPLAbort(env)
	env.TxnId = ""
	env.Instruction = ""
}
