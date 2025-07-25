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
	ColConfigContext     = "context"
	ColConfig            = "config"
	ColConfigDescription = "description"
	ColConfigUpdatedAt   = "updatedAt"
	ColConfigCreatedAt   = "createdAt"
)

func CreateSystemConfig(ctx context.Context, systemConfig shared.SystemConfig) error {
	now := shared.GetCurrentTime()
	systemConfig.CreatedAt = &now
	systemConfig.UpdatedAt = &now

	return services.DbPutItem(ctx, shared.ConfigTable, systemConfig)
}

func GetSystemConfig(ctx context.Context, context string) (shared.SystemConfig, error) {
	var systemConfig shared.SystemConfig
	err := services.DbGetItem(ctx, shared.ConfigTable, shared.SystemConfig{
		Context: context,
	}, &systemConfig)
	if err != nil {
		return shared.SystemConfig{}, err
	}
	return systemConfig, nil
}

func UpdateSystemConfig(ctx context.Context, systemConfig shared.SystemConfig) (shared.SystemConfig, error) {
	var update expression.UpdateBuilder

	// Check if any config field has values to update
	hasConfigUpdate := systemConfig.Config.SlackSettings.WebhookURL != "" ||
		systemConfig.Config.SlackSettings.Enabled != nil ||
		systemConfig.Config.EmailSettings.FromAddress != "" ||
		systemConfig.Config.EmailSettings.ReplyToAddress != "" ||
		systemConfig.Config.EmailSettings.Enabled != nil ||
		len(systemConfig.Config.InAppSettings.PlatformAppIDs) > 0 ||
		systemConfig.Config.InAppSettings.Enabled != nil

	if hasConfigUpdate {
		update = update.Set(expression.Name(ColConfig), expression.Value(systemConfig.Config))
	}
	if systemConfig.Description != "" {
		update = update.Set(expression.Name(ColConfigDescription), expression.Value(systemConfig.Description))
	}

	update = update.Set(expression.Name(ColConfigUpdatedAt), expression.Value(shared.GetCurrentTime()))

	out, err := services.DbUpdateItem(ctx, services.DbUpdateItemInput{
		TableName: shared.ConfigTable,
		Update:    update,
		Query: shared.SystemConfig{
			Context: systemConfig.Context,
		},
		Condition: expression.Name(ColConfigContext).Equal(expression.Value(systemConfig.Context)),
	})
	if err != nil {
		return shared.SystemConfig{}, err
	}

	var updatedSystemConfig shared.SystemConfig
	err = attributevalue.UnmarshalMap(out.Attributes, &updatedSystemConfig)
	if err != nil {
		return shared.SystemConfig{}, err
	}

	return updatedSystemConfig, nil
}

func GetSystemConfigList(ctx context.Context, limit int, startKey string) ([]shared.SystemConfig, string, error) {
	var lastEvaluatedKey map[string]types.AttributeValue
	var err error
	if startKey != "" {
		lastEvaluatedKey, err = attributevalue.MarshalMap(map[string]any{
			ColConfigContext: startKey,
		})
		if err != nil {
			return nil, "", err
		}
	}

	var items []shared.SystemConfig
	lastEvaluatedKey, err = services.DbScanItems(ctx, shared.ConfigTable, nil, nil, lastEvaluatedKey, limit, &items)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if lastEvaluatedKey != nil && lastEvaluatedKey[ColConfigContext] != nil {
		nextToken = lastEvaluatedKey[ColConfigContext].(*types.AttributeValueMemberS).Value
	}

	return items, nextToken, nil
}

func DeleteSystemConfig(ctx context.Context, context string) error {
	return services.DbDeleteItem(ctx, shared.ConfigTable, shared.SystemConfig{
		Context: context,
	})
}
