package db

import (
	"context"
	"notification-service/functions/services"
	"notification-service/functions/shared"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	ColUserID = "userId"
)

func GetUsersList(ctx context.Context, limit int, startKey string) ([]shared.User, *string, error) {
	// Handle pagination
	var lastEvaluatedKey map[string]types.AttributeValue
	if startKey != "" {
		lastEvaluatedKey = map[string]types.AttributeValue{
			ColUserID: &types.AttributeValueMemberS{
				Value: startKey,
			},
		}
	}

	var users []shared.User
	lastEvaluatedKey, err := services.DbScanItems(ctx, shared.UsersTable, nil, nil, lastEvaluatedKey, limit, &users)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to scan users table")
		return nil, nil, err
	}

	var nextKey *string
	if lastEvaluatedKey != nil {
		if userID, ok := lastEvaluatedKey[ColUserID]; ok {
			if userIDVal, ok := userID.(*types.AttributeValueMemberS); ok {
				nextKey = &userIDVal.Value
			}
		}
	}

	return users, nextKey, nil
}

func GetUserByID(ctx context.Context, userID string) (*shared.User, error) {

	var result shared.User
	err := services.DbGetItem(ctx, shared.UsersTable, shared.User{UserID: userID}, &result)
	if err != nil {
		shared.LogError().Err(err).Str("userId", userID).Msg("Failed to get user")
		return nil, err
	}

	if result.UserID == "" {
		return nil, nil
	}

	return &result, nil
}
