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
	ColPreferencesContext   = "context"
	ColPreferences          = "preferences"
	ColTimezone             = "timezone"
	ColLanguage             = "language"
	ColPreferencesUpdatedAt = "updatedAt"
)

func CreateUserPreferences(ctx context.Context, userPreferences shared.UserPreferences) error {
	now := shared.GetCurrentTime()
	userPreferences.CreatedAt = &now
	userPreferences.UpdatedAt = &now

	return services.DbPutItem(ctx, shared.PreferencesTable, userPreferences)
}

func GetUserPreferences(ctx context.Context, context string) (shared.UserPreferences, error) {
	var userPreferences shared.UserPreferences
	err := services.DbGetItem(ctx, shared.PreferencesTable, shared.UserPreferences{
		Context: context,
	}, &userPreferences)
	if err != nil {
		return shared.UserPreferences{}, err
	}
	return userPreferences, nil
}

func UpdateUserPreferences(ctx context.Context, userPreferences shared.UserPreferences) (shared.UserPreferences, error) {
	var update expression.UpdateBuilder

	if userPreferences.Preferences != nil {
		update = update.Set(expression.Name(ColPreferences), expression.Value(userPreferences.Preferences))
	}
	if userPreferences.Timezone != "" {
		update = update.Set(expression.Name(ColTimezone), expression.Value(userPreferences.Timezone))
	}
	if userPreferences.Language != "" {
		update = update.Set(expression.Name(ColLanguage), expression.Value(userPreferences.Language))
	}

	update = update.Set(expression.Name(ColPreferencesUpdatedAt), expression.Value(shared.GetCurrentTime()))

	out, err := services.DbUpdateItem(ctx, services.DbUpdateItemInput{
		TableName: shared.PreferencesTable,
		Update:    update,
		Query: shared.UserPreferences{
			Context: userPreferences.Context,
		},
		Condition: expression.Name(ColPreferencesContext).Equal(expression.Value(userPreferences.Context)),
	})
	if err != nil {
		return shared.UserPreferences{}, err
	}

	var updatedUserPreferences shared.UserPreferences
	err = attributevalue.UnmarshalMap(out.Attributes, &updatedUserPreferences)
	if err != nil {
		return shared.UserPreferences{}, err
	}

	return updatedUserPreferences, nil
}

func GetUserPreferencesList(ctx context.Context, limit int, startKey string) ([]shared.UserPreferences, string, error) {
	var lastEvaluatedKey map[string]types.AttributeValue
	var err error
	if startKey != "" {
		lastEvaluatedKey, err = attributevalue.MarshalMap(map[string]any{
			ColPreferencesContext: startKey,
		})
		if err != nil {
			return nil, "", err
		}
	}

	var items []shared.UserPreferences
	lastEvaluatedKey, err = services.DbScanItems(ctx, shared.PreferencesTable, nil, nil, lastEvaluatedKey, limit, &items)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if lastEvaluatedKey != nil && lastEvaluatedKey[ColPreferencesContext] != nil {
		nextToken = lastEvaluatedKey[ColPreferencesContext].(*types.AttributeValueMemberS).Value
	}

	return items, nextToken, nil
}

func DeleteUserPreferences(ctx context.Context, context string) error {
	return services.DbDeleteItem(ctx, shared.PreferencesTable, shared.UserPreferences{
		Context: context,
	})
}
