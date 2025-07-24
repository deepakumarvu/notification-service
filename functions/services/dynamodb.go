package services

import (
	"context"
	"notification-service/functions/shared"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func DbPutItem(ctx context.Context, tableName string, item any) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	_, err = shared.DynamoDBClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	return err
}

func DbGetItem(ctx context.Context, tableName string, query any, out any) error {
	av, err := attributevalue.MarshalMap(query)
	if err != nil {
		return err
	}

	shared.LogInfo().Str("tableName", tableName).Any("query", av).Msg("Getting item")

	result, err := shared.DynamoDBClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       av,
	})
	if err != nil {
		return err
	}
	return attributevalue.UnmarshalMap(result.Item, out)
}

func DbScanItems(ctx context.Context, tableName string, filterRows *expression.ConditionBuilder, outputColumns *expression.ProjectionBuilder, lastEvaluatedKey map[string]types.AttributeValue, limit int, out interface{}) (map[string]types.AttributeValue, error) {
	bldr := expression.NewBuilder()
	if filterRows != nil {
		bldr = bldr.WithFilter(*filterRows)
	}
	if outputColumns != nil {
		bldr = bldr.WithProjection(*outputColumns)
	}
	scanInput := dynamodb.ScanInput{
		TableName: aws.String(tableName),
	}

	if filterRows != nil || outputColumns != nil {
		expr, err := bldr.Build()
		if err != nil {
			return nil, err
		}
		scanInput.ExpressionAttributeNames = expr.Names()
		scanInput.ExpressionAttributeValues = expr.Values()

		if filterRows != nil {
			scanInput.FilterExpression = expr.Filter()
		}
		if outputColumns != nil {
			scanInput.ProjectionExpression = expr.Projection()
		}
	}
	if lastEvaluatedKey != nil {
		scanInput.ExclusiveStartKey = lastEvaluatedKey
	}
	if limit != 0 {
		scanInput.Limit = aws.Int32(int32(limit))
	}

	result, err := shared.DynamoDBClient.Scan(ctx, &scanInput)
	if err != nil {
		return nil, err
	}
	err = attributevalue.UnmarshalListOfMaps(result.Items, out)
	return result.LastEvaluatedKey, err
}

type DbUpdateItemInput struct {
	TableName string
	Update    expression.UpdateBuilder
	Query     any
	Condition expression.ConditionBuilder
}

// DbUpdateItem updates the items in the database
func DbUpdateItem(ctx context.Context, input DbUpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	keys, err := attributevalue.MarshalMap(input.Query)
	if err != nil {
		return nil, err
	}

	expr, err := expression.NewBuilder().
		WithCondition(input.Condition).
		WithUpdate(input.Update).
		Build()
	if err != nil {
		return nil, err
	}

	return shared.DynamoDBClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Key:                       keys,
		TableName:                 aws.String(input.TableName),
		UpdateExpression:          expr.Update(),
		ReturnValues:              types.ReturnValueAllNew,
		ConditionExpression:       expr.Condition(),
	})
}

/* Query based on some conditions on the composite keys */
func DbQuery(ctx context.Context, tableName, indexName string, limit int, startKey map[string]types.AttributeValue, expr expression.Expression, out interface{}, sortOrder *bool) (map[string]types.AttributeValue, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:                 aws.String(tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
	}

	if indexName != "" {
		queryInput.IndexName = &indexName
	}

	if limit != 0 {
		queryInput.Limit = aws.Int32(int32(limit))
	}

	if startKey != nil {
		queryInput.ExclusiveStartKey = startKey
	}

	if sortOrder != nil {
		queryInput.ScanIndexForward = sortOrder
	}

	result, err := shared.DynamoDBClient.Query(ctx, queryInput)
	if err != nil {
		return nil, err
	}
	return result.LastEvaluatedKey, attributevalue.UnmarshalListOfMaps(result.Items, out)
}

func DbDeleteItem(ctx context.Context, tableName string, query any) error {
	keys, err := attributevalue.MarshalMap(query)
	if err != nil {
		return err
	}

	_, err = shared.DynamoDBClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       keys,
	})
	return err
}
