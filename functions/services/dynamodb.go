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
