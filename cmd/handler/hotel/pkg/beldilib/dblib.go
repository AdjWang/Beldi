package beldilib

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	trace "github.com/eniac/Beldi/pkg/trace"
	"github.com/mitchellh/mapstructure"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/lithammer/shortuuid"
)

/**
The structure of a row looks like
| K | ROWHASH | V | LOGS | LOGSIZE | GCSIZE | NEXTROW
K and ROWHASH together act as Primary Key
K and V are the columns that developers/users operate on
All others are invisible to users
*/

var RESERVED = []string{"K", "ROWHASH", "LOGS", "LOGSIZE", "GCSIZE", "NEXTROW"}

func LibRead(tablename string, key aws.JSONValue, projection []string) aws.JSONValue {
	Key, err := dynamodbattribute.MarshalMap(key)
	CHECK(err)
	var res *dynamodb.GetItemOutput
	if len(projection) == 0 {
		res, err = DBClient.GetItem(&dynamodb.GetItemInput{
			TableName:      aws.String(tablename),
			Key:            Key,
			ConsistentRead: aws.Bool(true),
		})
	} else {
		expr, err := expression.NewBuilder().WithProjection(BuildProjection(projection)).Build()
		CHECK(err)
		res, err = DBClient.GetItem(&dynamodb.GetItemInput{
			TableName:                aws.String(tablename),
			Key:                      Key,
			ProjectionExpression:     expr.Projection(),
			ExpressionAttributeNames: expr.Names(),
			ConsistentRead:           aws.Bool(true),
		})
	}
	CHECK(err)
	item := aws.JSONValue{}
	err = dynamodbattribute.UnmarshalMap(res.Item, &item)
	CHECK(err)
	return item
}

// Put only if the key not exists.
func LibPut(tablename string, key aws.JSONValue, values aws.JSONValue) bool {
	Key, err := dynamodbattribute.MarshalMap(key)
	CHECK(err)

	updateBuilder := expression.UpdateBuilder{}
	condBuilder := expression.Value(0).Equal(expression.Value(0))
	for k, _ := range key {
		condBuilder = condBuilder.And(expression.AttributeNotExists(expression.Name(k)))
	}
	for k, v := range values {
		updateBuilder = updateBuilder.Set(expression.Name(k), expression.Value(v))
	}
	builder := expression.NewBuilder().WithCondition(condBuilder)
	if len(values) != 0 {
		builder = builder.WithUpdate(updateBuilder)
	}
	expr, err := builder.Build()
	CHECK(err)
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	if err == nil {
		return true
	} else {
		// AssertConditionFailure(errors.Wrapf(err, "TableName: %v, Key: %v", tablename, Key))
		AssertConditionFailure(err)
		return false // Failed because the key exists.
	}
}

// Put without any check.
func LibWrite(tablename string, key aws.JSONValue, update map[expression.NameBuilder]expression.OperandBuilder) {
	Key, err := dynamodbattribute.MarshalMap(key)
	CHECK(err)
	if len(update) == 0 {
		panic("update never be empty")
	}
	updateBuilder := expression.UpdateBuilder{}
	for k, v := range update {
		updateBuilder = updateBuilder.Set(k, v)
	}
	builder := expression.NewBuilder().WithUpdate(updateBuilder)
	expr, err := builder.Build()
	CHECK(err)
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	CHECK(err)
}

func LibCondWrite(tablename string, key string, update map[expression.NameBuilder]expression.OperandBuilder,
	cond expression.ConditionBuilder) bool {
	pk := aws.JSONValue{"K": key}
	Key, err := dynamodbattribute.MarshalMap(pk)
	updateBuilder := expression.UpdateBuilder{}
	for k, v := range update {
		updateBuilder = updateBuilder.Set(k, v)
	}
	expr, err := expression.NewBuilder().
		WithCondition(cond).
		WithUpdate(updateBuilder).
		Build()
	CHECK(err)
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	if err != nil {
		AssertConditionFailure(err)
		return false
	} else {
		return true
	}
}

func LibScanWithLast(tablename string, projection []string, filter expression.ConditionBuilder,
	last map[string]*dynamodb.AttributeValue) []aws.JSONValue {
	var res *dynamodb.ScanOutput
	var err error
	if last == nil {
		if len(projection) == 0 {
			expr, err := expression.NewBuilder().WithFilter(filter).Build()
			CHECK(err)
			res, err = DBClient.Scan(&dynamodb.ScanInput{
				TableName:                 aws.String(tablename),
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
				FilterExpression:          expr.Filter(),
				ConsistentRead:            aws.Bool(true),
			})
		} else {
			expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(BuildProjection(projection)).Build()
			CHECK(err)
			res, err = DBClient.Scan(&dynamodb.ScanInput{
				TableName:                 aws.String(tablename),
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
				FilterExpression:          expr.Filter(),
				ProjectionExpression:      expr.Projection(),
				ConsistentRead:            aws.Bool(true),
			})
		}
	} else {
		if len(projection) == 0 {
			expr, err := expression.NewBuilder().WithFilter(filter).Build()
			CHECK(err)
			res, err = DBClient.Scan(&dynamodb.ScanInput{
				TableName:                 aws.String(tablename),
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
				FilterExpression:          expr.Filter(),
				ConsistentRead:            aws.Bool(true),
				ExclusiveStartKey:         last,
			})
		} else {
			expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(BuildProjection(projection)).Build()
			CHECK(err)
			res, err = DBClient.Scan(&dynamodb.ScanInput{
				TableName:                 aws.String(tablename),
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
				FilterExpression:          expr.Filter(),
				ProjectionExpression:      expr.Projection(),
				ConsistentRead:            aws.Bool(true),
				ExclusiveStartKey:         last,
			})
		}
	}
	CHECK(err)
	var item []aws.JSONValue
	err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &item)
	CHECK(err)
	if res.LastEvaluatedKey == nil || len(res.LastEvaluatedKey) == 0 {
		return item
	}
	fmt.Println("DEBUG: Exceed Scan limit")
	item = append(item, LibScanWithLast(tablename, projection, filter, res.LastEvaluatedKey)...)
	return item
}

func LibScan(tablename string, projection []string, filter expression.ConditionBuilder) []aws.JSONValue {
	return LibScanWithLast(tablename, projection, filter, nil)
	//var res *dynamodb.ScanOutput
	//var err error
	//if len(projection) == 0 {
	//	expr, err := expression.NewBuilder().WithFilter(filter).Build()
	//	CHECK(err)
	//	res, err = DBClient.Scan(&dynamodb.ScanInput{
	//		TableName:                 aws.String(tablename),
	//		ExpressionAttributeNames:  expr.Names(),
	//		ExpressionAttributeValues: expr.Values(),
	//		FilterExpression:          expr.Filter(),
	//		ConsistentRead:            aws.Bool(true),
	//	})
	//} else {
	//	expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(BuildProjection(projection)).Build()
	//	CHECK(err)
	//	res, err = DBClient.Scan(&dynamodb.ScanInput{
	//		TableName:                 aws.String(tablename),
	//		ExpressionAttributeNames:  expr.Names(),
	//		ExpressionAttributeValues: expr.Values(),
	//		FilterExpression:          expr.Filter(),
	//		ProjectionExpression:      expr.Projection(),
	//		ConsistentRead:            aws.Bool(true),
	//	})
	//}
	//CHECK(err)
	//var item []aws.JSONValue
	//err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &item)
	//CHECK(err)
	//return item
}

func LibDelete(tablename string, key aws.JSONValue) {
	Key, err := dynamodbattribute.MarshalMap(key)
	CHECK(err)
	param := &dynamodb.DeleteItemInput{
		TableName: aws.String(tablename),
		Key:       Key,
	}
	_, err = DBClient.DeleteItem(param)
	if err != nil {
		LibDelete(tablename, key)
	}
}

// 1. Read a value from `tablename` where the value is writen by EOSWrite.EOSWriteWithRow
// sometime.
//
// 2. Then check the env.LogTable if current Read had performed. If so, read from log.
//
// The 2., that to check if the read had been logged, should be executed first, or the
// Read operation in 1. would be a waste if the read is logged.
// It's so weird in Beldi and it has been optimized in Boki.
//
// Note that there are 2 tables: `tablename` and `env.LogTable`, where
// `tablename` is the table of Write logs, also the DAAL;
// `env.LogTable` is a separate table of Read logs, there's no DAAL here;
//
// The DAAL is a linked list, all the writes are performed in their time order, so the
// last write must be at the tail row, where this EOSReadWithRow should perform.
//
// Assume that most part of Reads are normal requests and will be performed on the DAAL,
// all of them are iterating to the tail, which is an unnecessary heavy overhead and
// can be optimized by a cyclic list.
// Besides, due to the read log(env.LogTable) is a separate table and no DAAL here, so
// there's no need to optimize the read key searching.
func EOSReadWithRow(env *Env, tablename string, key string, projection []string, row string) aws.JSONValue {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSReadWithRow table: %v, key: %v, row: %v", tablename, key, row))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	var metas []string
	if len(projection) == 0 {
		metas = []string{}
	} else {
		metas = append(projection, "NEXTROW")
	}
	res := LibRead(tablename, aws.JSONValue{"K": key, "ROWHASH": row}, metas)
	// read the last row of DAAL, where the last EOSWriteWithRow had performed at
	if nextRow, exists := res["NEXTROW"]; exists {
		return EOSReadWithRow(env, tablename, key, projection, nextRow.(string))
	}
	// last row read
	for _, column := range RESERVED {
		delete(res, column)
	}
	// res only leaves V now
	logKey := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}
	env.StepNumber += 1
	if LibPut(env.LogTable, logKey, res) {
		return res
	} else {
		return LibRead(env.LogTable, logKey, projection)
	}
}

func EOSRead(env *Env, tablename string, key string, projection []string) aws.JSONValue {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSRead table: %v, key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	// ReadLog is not in DAAL, Need Optimization Here
	last := LastRow(tablename, key)
	if last == "" {
		last = "HEAD"
	}
	return EOSReadWithRow(env, tablename, key, projection, last)
}

func LibReadLatest(tablename string, key string, projection []string, row string) aws.JSONValue {
	res := LibRead(tablename, aws.JSONValue{"K": key, "ROWHASH": row}, append(projection, "NEXTROW"))
	if nextRow, exists := res["NEXTROW"].(string); exists {
		return LibRead(tablename, aws.JSONValue{"K": key, "ROWHASH": nextRow}, projection)
	} else {
		return res
	}
}

func EOSScan(env *Env, tablename string, projection []string) []aws.JSONValue {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSScan table: %v", tablename))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	var res []aws.JSONValue
	keys := LibScan(tablename, []string{"K"}, expression.Name("ROWHASH").Equal(expression.Value("HEAD")))
	for _, key := range keys {
		res = append(res, LibReadLatest(tablename, key["K"].(string), projection, "HEAD"))
	}
	logKey := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}
	env.StepNumber += 1
	if LibPut(env.LogTable, logKey, aws.JSONValue{"VS": res}) {
		return res
	}
	item := LibRead(env.LogTable, logKey, []string{"VS"})
	CHECK(mapstructure.Decode(item["VS"], &res))
	return res
}

func InsertOrGetNewRow(tablename string, key string, row string) string {
	//fmt.Printf("Going to insert %s\n", row)
	pk, Key := GeneratePK(key, row)
	newRowHash := shortuuid.New()
	newPk, newKey := GeneratePK(key, newRowHash)

	newUpdateBuilder := expression.UpdateBuilder{}
	oldItem := LibRead(tablename, pk, []string{"V"})
	if val, ok := oldItem["V"]; ok {
		newUpdateBuilder = newUpdateBuilder.Set(expression.Name("V"), expression.Value(val))
	}
	newUpdateBuilder = newUpdateBuilder.
		Set(expression.Name("LOGS"),
			expression.Value(aws.JSONValue{"ignore": nil})).
		Set(expression.Name("LOGSIZE"),
			expression.Value(0)).
		Set(expression.Name("GCSIZE"),
			expression.Value(0))

	condBuilder := expression.And(
		expression.AttributeNotExists(expression.Name("K")),
		expression.AttributeNotExists(expression.Name("ROWHASH")))
	expr, err := expression.NewBuilder().WithCondition(condBuilder).WithUpdate(newUpdateBuilder).Build()
	CHECK(err)
	//fmt.Printf("Going to create a new row\n")
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       newKey,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	CHECK(err) // Creating a new row never fails
	//fmt.Printf("Create row success!!!!\n")

	swapUpdateBuilder := expression.Set(expression.Name("NEXTROW"), expression.Value(newRowHash))
	condBuilder = expression.AttributeNotExists(expression.Name("NEXTROW"))
	expr, err = expression.NewBuilder().WithCondition(condBuilder).WithUpdate(swapUpdateBuilder).Build()
	CHECK(err)
	//fmt.Printf("Going to swap row\n")
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	if err == nil {
		//fmt.Printf("Swap row success!!!!\n")
		return newRowHash
	} else {
		//fmt.Printf("Swap row failure!!!!\n")
		AssertConditionFailure(err)
		//fmt.Printf("Try delete tmp row\n")
		LibDelete(tablename, newPk)
		//fmt.Printf("Delete tmp row success\n")
		//last := LastRow(tablename, key)
		//return last
		res := LibRead(tablename, pk, []string{"NEXTROW"})
		if nextRow, exists := res["NEXTROW"].(string); exists {
			//fmt.Printf("Fetch a nextrow\n")
			return nextRow
		} else {
			panic("never reach here")
		}
	}
}

func InsertHead(tablename string, key string) {
	_, Key := GeneratePK(key, "HEAD")

	newUpdateBuilder := expression.UpdateBuilder{}
	newUpdateBuilder = newUpdateBuilder.
		Set(expression.Name("LOGS"),
			expression.Value(aws.JSONValue{"ignore": nil})).
		Set(expression.Name("LOGSIZE"),
			expression.Value(0)).
		Set(expression.Name("GCSIZE"),
			expression.Value(0))

	condBuilder := expression.And(
		expression.AttributeNotExists(expression.Name("K")),
		expression.AttributeNotExists(expression.Name("ROWHASH")))
	expr, err := expression.NewBuilder().WithCondition(condBuilder).WithUpdate(newUpdateBuilder).Build()
	CHECK(err)
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
}

func EOSWriteWithRow(env *Env, tablename string, key string,
	update map[expression.NameBuilder]expression.OperandBuilder, row string) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSWriteWithRow table: %v, key: %v, row: %v", tablename, key, row))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	pk, Key := GeneratePK(key, row)
	cid := fmt.Sprintf("%s-%v", env.InstanceId, env.StepNumber)
	cidPath := fmt.Sprintf("LOGS.%s", cid)

	cond1 := expression.AttributeNotExists(expression.Name(cidPath))           // CID not in logs
	cond2 := expression.Name("LOGSIZE").LessThan(expression.Value(GLOGSIZE())) // |logs| < N

	// CID not in logs /\ |logs| < N /\ not exist NextRow
	updateBuilder := expression.UpdateBuilder{}
	for k, v := range update {
		updateBuilder = updateBuilder.Set(k, v)
	}
	updateBuilder = updateBuilder.
		Set(expression.Name(cidPath), expression.Value(nil)).
		Set(expression.Name("LOGSIZE"),
			expression.Name("LOGSIZE").Plus(expression.Value(1)))

	expr, err := expression.NewBuilder().WithCondition(expression.And(cond1, cond2)).WithUpdate(updateBuilder).Build()
	CHECK(err)
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	if err == nil {
		env.StepNumber += 1
		return
	}
	AssertConditionFailure(err)
	// CID in logs
	res := LibRead(tablename, pk, []string{cidPath, "NEXTROW"})
	if res["LOGS"] != nil {
		env.StepNumber += 1
		return
	}
	if nextRow, exists := res["NEXTROW"].(string); exists {
		// CID not in logs /\ |logs| = N /\ exists NextRow
		EOSWriteWithRow(env, tablename, key, update, nextRow)
	} else {
		// CID not in logs /\ |logs| = N /\ not exist NextRow
		nextRow := InsertOrGetNewRow(tablename, key, row)
		EOSWriteWithRow(env, tablename, key, update, nextRow)
	}
}

func QueryCheck(env *Env, tablename string, key string, idx []string) bool {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("QueryCheck table: %v, key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	cid := fmt.Sprintf("%s-%v", env.InstanceId, env.StepNumber)
	cidPath := fmt.Sprintf("LOGS.%s", cid)
	filter := expression.Value(false).Equal(expression.Value(true))
	for _, row := range idx {
		filter = filter.Or(expression.Name("ROWHASH").Equal(expression.Value(row)))
	}
	keyCond := expression.Key("K").Equal(expression.Value(key))

	expr, err := expression.NewBuilder().
		WithProjection(BuildProjection([]string{cidPath})).
		WithKeyCondition(keyCond).
		WithFilter(filter).
		Build()
	CHECK(err)
	res, err := DBClient.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tablename),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//ConsistentRead:            aws.Bool(true),
	})
	CHECK(err)
	var items []aws.JSONValue
	err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &items)
	CHECK(err)
	if len(items) != 0 {
		env.StepNumber += 1
		return true
	}
	return false
}

// if done, res, last
// Returns:
//
//	bool: If the key had beed writen before.
//	bool: The value of cid in LOGS entry.
//	string: The key of the last row.
func QuickCheckReturnLast(env *Env, tablename string, key string, isCond bool) (bool, bool, string) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("QuickCheckReturnLast table: %v, key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	cid := fmt.Sprintf("%s-%v", env.InstanceId, env.StepNumber)
	cidPath := fmt.Sprintf("LOGS.%s", cid)
	projection := []string{"ROWHASH", "NEXTROW", cidPath}
	cond := expression.Key("K").Equal(expression.Value(key))
	expr, err := expression.NewBuilder().
		WithProjection(BuildProjection(projection)).WithKeyCondition(cond).Build()
	CHECK(err)
	res, err := DBClient.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tablename),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ConsistentRead:            aws.Bool(true),
	})
	CHECK(err)
	var items []aws.JSONValue
	err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &items)
	CHECK(err)
	if len(items) == 0 {
		return false, false, ""
	}
	idx := make(map[string]string)
	for _, item := range items {
		if res, ok := item["LOGS"]; ok {
			if isCond {
				return true, res.(map[string]interface{})[cid].(bool), ""
			} else {
				return true, false, ""
			}
		}
		row := item["ROWHASH"].(string)
		if next, ok := item["NEXTROW"].(string); ok {
			idx[row] = next
		}
	}
	cur := "HEAD"
	for {
		if next, ok := idx[cur]; ok {
			cur = next
			continue
		} else {
			break
		}
	}
	return false, false, cur
}

func QueryCondCheck(env *Env, tablename string, key string, idx []string) (bool, bool) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("QueryCheck table: %v, key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	cid := fmt.Sprintf("%s-%v", env.InstanceId, env.StepNumber)
	cidPath := fmt.Sprintf("LOGS.%s", cid)
	filter := expression.Value(false).Equal(expression.Value(true))
	for _, row := range idx {
		filter = filter.Or(expression.Name("ROWHASH").Equal(expression.Value(row)))
	}
	filter = expression.AttributeExists(expression.Name(cidPath)).And(filter)
	keyCond := expression.Key("K").Equal(expression.Value(key))

	expr, err := expression.NewBuilder().
		WithProjection(BuildProjection([]string{cidPath})).
		WithKeyCondition(keyCond).
		WithFilter(filter).
		Build()
	CHECK(err)
	res, err := DBClient.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tablename),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//ConsistentRead:            aws.Bool(true),
	})
	CHECK(err)
	var items []aws.JSONValue
	err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &items)
	CHECK(err)

	if len(items) != 0 {
		env.StepNumber += 1
		return true, items[0]["LOGS"].(map[string]interface{})[cid].(bool)
	}
	return false, true
}

func EOSWrite(env *Env, tablename string, key string,
	update map[expression.NameBuilder]expression.OperandBuilder) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSWrite table: %v, key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	done, _, last := QuickCheckReturnLast(env, tablename, key, false)
	if done {
		env.StepNumber += 1
		return
	}
	if last == "" {
		InsertHead(tablename, key)
		EOSWriteWithRow(env, tablename, key, update, "HEAD")
	} else {
		EOSWriteWithRow(env, tablename, key, update, last)
	}
}

func EOSDelete(env *Env, tablename string, key string) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSDelete table: %v, key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	EOSWrite(env, tablename, key, map[expression.NameBuilder]expression.OperandBuilder{
		expression.Name("V"): expression.Value(nil),
	})
}

func EOSCondWriteWithRow(env *Env, tablename string, key string,
	update map[expression.NameBuilder]expression.OperandBuilder, cond expression.ConditionBuilder, row string) bool {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSCondWriteWithRow table: %v, key: %v, row: %v", tablename, key, row))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	pk := aws.JSONValue{"K": key, "ROWHASH": row}
	Key, err := dynamodbattribute.MarshalMap(pk)
	CHECK(err)
	cid := fmt.Sprintf("%s-%v", env.InstanceId, env.StepNumber)
	cidPath := fmt.Sprintf("LOGS.%s", cid)

	cond1 := expression.AttributeNotExists(expression.Name(cidPath))           // CID not in logs
	cond2 := expression.Name("LOGSIZE").LessThan(expression.Value(GLOGSIZE())) // |logs| < N

	// CID not in logs /\ |logs| < N /\ not exist NextRow
	updateBuilder := expression.UpdateBuilder{}
	for k, v := range update {
		updateBuilder = updateBuilder.Set(k, v)
	}
	successUpdateBuilder := updateBuilder.
		Set(expression.Name("LOGSIZE"),
			expression.Name("LOGSIZE").Plus(expression.Value(1))).
		Set(expression.Name(cidPath), expression.Value(true))

	failureUpdateBuilder := expression.Set(expression.Name(cidPath), expression.Value(false))

	expr, err := expression.NewBuilder().
		WithCondition(expression.And(cond1, cond2, cond)).
		WithUpdate(successUpdateBuilder).
		Build()
	CHECK(err)
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	if err == nil {
		env.StepNumber += 1
		return true
	}

	expr, err = expression.NewBuilder().
		WithCondition(expression.And(cond1, cond2)).
		WithUpdate(failureUpdateBuilder).
		Build()
	CHECK(err)
	_, err = DBClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName:                 aws.String(tablename),
		Key:                       Key,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	})
	if err == nil {
		env.StepNumber += 1
		return false
	}
	AssertConditionFailure(err)

	// CID in logs
	res := LibRead(tablename, pk, []string{cidPath, "NEXTROW"})
	if res["LOGS"] != nil {
		env.StepNumber += 1
		return res["LOGS"].(map[string]interface{})[cid].(bool)
	}
	if nextRow, exists := res["NEXTROW"].(string); exists {
		// CID not in logs /\ |logs| = N /\ exists NextRow
		return EOSCondWriteWithRow(env, tablename, key, update, cond, nextRow)
	} else {
		// CID not in logs /\ |logs| = N /\ not exist NextRow
		nextRow := InsertOrGetNewRow(tablename, key, row)
		return EOSCondWriteWithRow(env, tablename, key, update, cond, nextRow)
	}
}

func EOSCondWrite(env *Env, tablename string, key string,
	update map[expression.NameBuilder]expression.OperandBuilder,
	cond expression.ConditionBuilder) bool {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("EOSCondWriteWithRow table: %v, key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	done, res, last := QuickCheckReturnLast(env, tablename, key, true)
	if done {
		env.StepNumber += 1
		return res
	}
	if last == "" {
		InsertHead(tablename, key)
		return EOSCondWriteWithRow(env, tablename, key, update, cond, "HEAD")
	} else {
		return EOSCondWriteWithRow(env, tablename, key, update, cond, last)
	}
}

func Read(env *Env, tablename string, key string) interface{} {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("Read table: %v key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	var item aws.JSONValue
	if TYPE == "BASELINE" {
		item = LibRead(tablename, aws.JSONValue{"K": key}, []string{"V"})
	} else {
		item = EOSRead(env, tablename, key, []string{"V"})
	}
	if res, ok := item["V"]; ok {
		return res
	} else {
		return nil
	}
}

func Write(env *Env, tablename string, key string,
	update map[expression.NameBuilder]expression.OperandBuilder) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("Write table: %v key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	if TYPE == "BASELINE" {
		LibWrite(tablename, aws.JSONValue{"K": key}, update)
	} else {
		EOSWrite(env, tablename, key, update)
	}
}

func CondWrite(env *Env, tablename string, key string,
	update map[expression.NameBuilder]expression.OperandBuilder, cond expression.ConditionBuilder) bool {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("CondWrite table: %v key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	if TYPE == "BASELINE" {
		return LibCondWrite(tablename, key, update, cond)
	} else {
		return EOSCondWrite(env, tablename, key, update, cond)
	}
}

func Scan(env *Env, tablename string) interface{} {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("Scan table: %v", tablename))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	if TYPE == "BASELINE" {
		var res []interface{}
		items := LibScan(tablename, []string{"V"}, expression.Value(true).Equal(expression.Value(true)))
		for _, item := range items {
			res = append(res, item["V"])
		}
		return res
	}
	var res []interface{}
	items := LibScan(tablename, []string{"V"},
		expression.AttributeNotExists(expression.Name("NEXTROW")))
	for _, item := range items {
		res = append(res, item["V"])
	}
	logKey := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}
	env.StepNumber += 1
	if LibPut(env.LogTable, logKey, aws.JSONValue{"VS": res}) {
		return res
	}
	item := LibRead(env.LogTable, logKey, []string{"VS"})
	return item["VS"]
}

func TRead(env *Env, tablename string, key string) aws.JSONValue {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("TRead table: %v key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	item := LibRead(tablename, aws.JSONValue{"K": key}, []string{"V"})
	logKey := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}
	env.StepNumber += 1
	if LibPut(env.LogTable, logKey, item) {
		return item
	}
	return LibRead(env.LogTable, logKey, []string{"V"})
}

func TWrite(env *Env, tablename string, key string, value string) {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("TWrite table: %v key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	logItem, err := dynamodbattribute.MarshalMap(aws.JSONValue{
		"InstanceId": env.InstanceId,
		"StepNumber": env.StepNumber,
	})
	CHECK(err)
	K, err := dynamodbattribute.MarshalMap(aws.JSONValue{
		"K": key,
	})
	CHECK(err)
	update, err := dynamodbattribute.MarshalMap(aws.JSONValue{
		":V": value,
	})
	CHECK(err)
	for {
		_, err = DBClient.TransactWriteItems(&dynamodb.TransactWriteItemsInput{
			TransactItems: []*dynamodb.TransactWriteItem{
				&dynamodb.TransactWriteItem{
					Put: &dynamodb.Put{
						ConditionExpression: aws.String("attribute_not_exists(InstanceId) and attribute_not_exists(StepNumber)"),
						Item:                logItem,
						TableName:           aws.String(env.LogTable),
					},
				},
				&dynamodb.TransactWriteItem{Update: &dynamodb.Update{
					Key:                       K,
					ExpressionAttributeValues: update,
					TableName:                 aws.String(tablename),
					UpdateExpression:          aws.String("Set V = :V"),
				}},
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), "ConditionalCheckFailed") {
				break
			}
			if strings.Contains(err.Error(), "Conflict") {
				continue
			}
			panic(err)
		}
		break
	}
	env.StepNumber += 1
}

func TCondWrite(env *Env, tablename string, key string, value string, c bool) bool {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("TCondWrite table: %v key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	logItem, err := dynamodbattribute.MarshalMap(aws.JSONValue{
		"InstanceId": env.InstanceId,
		"StepNumber": env.StepNumber,
		"Res":        true,
	})
	CHECK(err)
	K, err := dynamodbattribute.MarshalMap(aws.JSONValue{
		"K": key,
	})
	CHECK(err)
	update, err := dynamodbattribute.MarshalMap(aws.JSONValue{
		":V": value,
		":A": 1,
		":B": 1,
	})
	CHECK(err)
	done := false
	var cond string
	if c {
		cond = ":A = :B"
	} else {
		cond = ":A < :B"
	}
	for {
		_, err = DBClient.TransactWriteItems(&dynamodb.TransactWriteItemsInput{
			TransactItems: []*dynamodb.TransactWriteItem{
				&dynamodb.TransactWriteItem{
					Put: &dynamodb.Put{
						ConditionExpression: aws.String("attribute_not_exists(InstanceId) and attribute_not_exists(StepNumber)"),
						Item:                logItem,
						TableName:           aws.String(env.LogTable),
					},
				},
				&dynamodb.TransactWriteItem{Update: &dynamodb.Update{
					Key:                       K,
					ExpressionAttributeValues: update,
					TableName:                 aws.String(tablename),
					UpdateExpression:          aws.String("Set V = :V"),
					ConditionExpression:       aws.String(cond),
				}},
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), "[ConditionalCheckFailed") {
				item := LibRead(env.LogTable, aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}, []string{"res"})
				return item["Res"].(bool)
			}
			if strings.Contains(err.Error(), "ConditionalCheckFailed]") {
				done = false
				break
			}
			if strings.Contains(err.Error(), "Conflict") {
				continue
			}
			panic(err)
		}
		done = true
		break
	}
	if done {
		env.StepNumber += 1
		return true
	}
	logItem, err = dynamodbattribute.MarshalMap(aws.JSONValue{
		"InstanceId": env.InstanceId,
		"StepNumber": env.StepNumber,
		"Res":        false,
	})
	ok := LibPut(env.LogTable, aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}, aws.JSONValue{"res": false})
	if ok {
		env.StepNumber += 1
		return false
	} else {
		item := LibRead(env.LogTable, aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}, []string{"res"})
		env.StepNumber += 1
		return item["Res"].(bool)
	}
}

func LibQuery(tablename string, cond expression.KeyConditionBuilder, projection []string) []aws.JSONValue {
	expr, err := expression.NewBuilder().WithProjection(BuildProjection(projection)).WithKeyCondition(cond).Build()
	CHECK(err)
	res, err := DBClient.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tablename),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ConsistentRead:            aws.Bool(true),
	})
	if err != nil {
		//fmt.Print("LIBQUERY ERROR: ")
		//fmt.Println(err)
		return LibQuery(tablename, cond, projection)
		//return []aws.JSONValue{}
	}
	var items []aws.JSONValue
	err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &items)
	CHECK(err)
	return items
}

func LastRow(tablename string, key string) string {
	projection := []string{"ROWHASH", "NEXTROW"}
	cond := expression.Key("K").Equal(expression.Value(key))
	expr, err := expression.NewBuilder().WithProjection(BuildProjection(projection)).WithKeyCondition(cond).Build()
	CHECK(err)
	res, err := DBClient.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tablename),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ConsistentRead:            aws.Bool(true),
	})
	CHECK(err)
	var items []aws.JSONValue
	err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &items)
	CHECK(err)
	if len(items) == 0 {
		return ""
	}
	idx := make(map[string]string)
	for _, item := range items {
		row := item["ROWHASH"].(string)
		if next, ok := item["NEXTROW"].(string); ok {
			idx[row] = next
		}
	}
	cur := "HEAD"
	for {
		if next, ok := idx[cur]; ok {
			cur = next
			continue
		} else {
			break
		}
	}
	return cur
}

func TQuery(env *Env, tablename string, key string) interface{} {
	ctx, span := trace.NewSpan(env.Ctx, fmt.Sprintf("TQuery table: %v key: %v", tablename, key))
	originalCtx := env.Ctx
	env.Ctx = ctx
	defer func() {
		span.End()
		env.Ctx = originalCtx
	}()

	projection := []string{"ROWHASH", "V", "NEXTROW"}
	cond := expression.Key("K").Equal(expression.Value(key))
	expr, err := expression.NewBuilder().WithProjection(BuildProjection(projection)).WithKeyCondition(cond).Build()
	CHECK(err)
	res, err := DBClient.Query(&dynamodb.QueryInput{
		TableName:                 aws.String(tablename),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//ConsistentRead:            aws.Bool(true),
	})
	CHECK(err)
	var items []aws.JSONValue
	err = dynamodbattribute.UnmarshalListOfMaps(res.Items, &items)
	CHECK(err)
	idx := make(map[string]aws.JSONValue)
	for _, item := range items {
		row := item["ROWHASH"].(string)
		tmp := aws.JSONValue{}
		if next, ok := item["NEXTROW"]; ok {
			tmp["NEXTROW"] = next
		}
		if v, ok := item["V"]; ok {
			tmp["V"] = v
		}
		idx[row] = tmp
	}
	cur := "HEAD"
	var v map[string]interface{} = nil
	for {
		v = idx[cur]
		if next, ok := v["NEXTROW"]; ok {
			cur = next.(string)
			continue
		} else {
			break
		}
	}
	logKey := aws.JSONValue{"InstanceId": env.InstanceId, "StepNumber": env.StepNumber}
	env.StepNumber += 1
	if LibPut(env.LogTable, logKey, v) {
		return v
	}
	return LibRead(env.LogTable, logKey, projection)
}

func PrintExp(exp expression.Expression) {
	fmt.Println("Names:")
	for k, v := range exp.Names() {
		fmt.Printf("%s %s\n", k, *v)
	}
	fmt.Println("----------")
	fmt.Println("Values:")
	for k, v := range exp.Values() {
		fmt.Printf("%s %s\n", k, *v)
	}
	if exp.Filter() != nil {
		fmt.Println("----------")
		fmt.Printf("Filter: %s\n", *exp.Filter())
	}
	if exp.Update() != nil {
		fmt.Println("----------")
		fmt.Printf("Update: %s\n", *exp.Update())
	}
	if exp.Condition() != nil {
		fmt.Println("----------")
		fmt.Printf("Condition: %s\n", *exp.Condition())
	}
	if exp.Projection() != nil {
		fmt.Println("----------")
		fmt.Printf("Projection: %s\n", *exp.Projection())
	}
}

func AssertConditionFailure(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case dynamodb.ErrCodeConditionalCheckFailedException:
			return
		default:
			fmt.Println(fmt.Sprintf("ERROR: %s", aerr))
			panic("ERROR detected")
		}
	} else {
		fmt.Println(fmt.Sprintf("ERROR: %s", err))
		panic("ERROR detected")
	}
}

func GeneratePK(k string, rowHash string) (aws.JSONValue, map[string]*dynamodb.AttributeValue) {
	pk := aws.JSONValue{"K": k, "ROWHASH": rowHash}
	Key, err := dynamodbattribute.MarshalMap(pk)
	CHECK(err)
	return pk, Key
}

func BuildProjection(names []string) expression.ProjectionBuilder {
	if len(names) == 0 {
		panic("Projection must > 0")
	}
	var builder expression.ProjectionBuilder
	for _, name := range names {
		builder = builder.AddNames(expression.Name(name))
	}
	return builder
}

func ListTables() {
	// create the input configuration instance
	input := &dynamodb.ListTablesInput{}

	fmt.Printf("Tables:\n")

	for {
		// Get the list of tables
		result, err := DBClient.ListTables(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case dynamodb.ErrCodeInternalServerError:
					fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
			return
		}

		for _, n := range result.TableNames {
			fmt.Println(*n)
		}

		// assign the last read tablename as the start for our next call to the ListTables function
		// the maximum number of table names returned in a call is 100 (default), which requires us to make
		// multiple calls to the ListTables function to retrieve all table names
		input.ExclusiveStartTableName = result.LastEvaluatedTableName

		if result.LastEvaluatedTableName == nil {
			break
		}
	}
}
