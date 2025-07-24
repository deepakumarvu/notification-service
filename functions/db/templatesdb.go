package db

import (
	"context"
	"notification-service/functions/services"
	"notification-service/functions/shared"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	TemplateActive = true
	ColTypeChannel = "type#channel"
	ColContext     = "context"
	ColUpdatedAt   = "updatedAt"
	ColContent     = "content"
	ColIsActive    = "isActive"
)

func CreateTemplate(ctx context.Context, template shared.Template) error {
	now := shared.GetCurrentTime()
	template.CreatedAt = &now
	template.UpdatedAt = &now

	return services.DbPutItem(ctx, shared.TemplatesTable, template)
}

func GetTemplateByTypeChannel(ctx context.Context, context, typeChannel string) (shared.Template, error) {
	var template shared.Template
	err := services.DbGetItem(ctx, shared.TemplatesTable, shared.Template{
		Context:     context,
		TypeChannel: typeChannel,
	}, &template)
	if err != nil {
		return shared.Template{}, err
	}
	return template, nil
}

func UpdateTemplate(ctx context.Context, template shared.Template) (shared.Template, error) {

	var update expression.UpdateBuilder

	if template.Content != "" {
		update = update.Set(expression.Name(ColContent), expression.Value(template.Content))
	}
	if template.IsActive != nil {
		update = update.Set(expression.Name(ColIsActive), expression.Value(template.IsActive))
	}

	update = update.Set(expression.Name(ColUpdatedAt), expression.Value(shared.GetCurrentTime()))

	out, err := services.DbUpdateItem(ctx, services.DbUpdateItemInput{
		TableName: shared.TemplatesTable,
		Update:    update,
		Query: shared.Template{
			Context:     template.Context,
			TypeChannel: template.TypeChannel,
		},
		Condition: expression.Name(ColTypeChannel).Equal(expression.Value(template.TypeChannel)).
			And(expression.Name(ColContext).Equal(expression.Value(template.Context))),
	})
	if err != nil {
		return shared.Template{}, err
	}

	var updatedTemplate shared.Template
	err = attributevalue.UnmarshalMap(out.Attributes, &updatedTemplate)
	if err != nil {
		return shared.Template{}, err
	}

	return updatedTemplate, nil
}

func GetTemplatesList(ctx context.Context, context string, limit int, startKey string) ([]shared.Template, string, error) {

	keyCondition := expression.KeyEqual(expression.Key("context"), expression.Value(context))

	expr, errExpressionBuilder := expression.NewBuilder().
		WithKeyCondition(keyCondition).
		Build()
	if errExpressionBuilder != nil {
		return nil, "", errExpressionBuilder
	}
	var lastEvaluatedKey map[string]types.AttributeValue
	var err error
	if startKey != "" {
		lastEvaluatedKey, err = attributevalue.MarshalMap(map[string]any{
			"context":      context,
			"type#channel": startKey,
		})
		if err != nil {
			return nil, "", err
		}
	}

	var items []shared.Template
	nextKey, err := services.DbQuery(ctx, shared.TemplatesTable, "", limit, lastEvaluatedKey, expr, &items, nil)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if nextKey != nil && nextKey["type#channel"] != nil {
		nextToken = nextKey["type#channel"].(*types.AttributeValueMemberS).Value
	}

	return items, nextToken, nil
}

func DeleteTemplate(ctx context.Context, context, typeChannel string) error {
	return services.DbDeleteItem(ctx, shared.TemplatesTable, shared.Template{
		Context:     context,
		TypeChannel: typeChannel,
	})
}
