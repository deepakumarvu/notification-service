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
	ColScheduleID        = "scheduleId"
	ColScheduleUserID    = "userId"
	ColScheduleType      = "type"
	ColScheduleVariables = "variables"
	ColScheduleConfig    = "schedule"
	ColScheduleStatus    = "status"
	ColScheduleCreatedAt = "createdAt"
	ColScheduleUpdatedAt = "updatedAt"
)

func CreateScheduledNotification(ctx context.Context, notification shared.ScheduledNotification) error {
	now := shared.GetCurrentTime()
	notification.CreatedAt = &now
	notification.UpdatedAt = &now
	notification.Status = shared.StatusActive

	return services.DbPutItem(ctx, shared.SchedulesTable, notification)
}

func GetScheduledNotification(ctx context.Context, scheduleID string) (shared.ScheduledNotification, error) {
	var notification shared.ScheduledNotification
	err := services.DbGetItem(ctx, shared.SchedulesTable, shared.ScheduledNotification{
		ScheduleID: scheduleID,
	}, &notification)
	if err != nil {
		return shared.ScheduledNotification{}, err
	}
	return notification, nil
}

func GetUserScheduledNotifications(ctx context.Context, userID string, limit int, startKey string) ([]shared.ScheduledNotification, string, error) {
	var lastEvaluatedKey map[string]types.AttributeValue
	var err error
	if startKey != "" {
		lastEvaluatedKey, err = attributevalue.MarshalMap(map[string]any{
			ColScheduleUserID:    userID,
			ColScheduleCreatedAt: startKey,
		})
		if err != nil {
			return nil, "", err
		}
	}

	// Create key condition for UserIndex GSI
	keyCondition := expression.Key(ColScheduleUserID).Equal(expression.Value(userID))
	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		return nil, "", err
	}

	var items []shared.ScheduledNotification
	lastEvaluatedKey, err = services.DbQuery(ctx, shared.SchedulesTable, "UserIndex", limit, lastEvaluatedKey, expr, &items, nil)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if lastEvaluatedKey != nil && lastEvaluatedKey[ColScheduleCreatedAt] != nil {
		nextToken = lastEvaluatedKey[ColScheduleCreatedAt].(*types.AttributeValueMemberS).Value
	}

	return items, nextToken, nil
}

func UpdateScheduledNotification(ctx context.Context, notification shared.ScheduledNotification) (shared.ScheduledNotification, error) {
	var update expression.UpdateBuilder

	if notification.Status != "" {
		update = update.Set(expression.Name(ColScheduleStatus), expression.Value(notification.Status))
	}
	if notification.Variables != nil {
		update = update.Set(expression.Name(ColScheduleVariables), expression.Value(notification.Variables))
	}
	if notification.Schedule.Type != "" {
		update = update.Set(expression.Name(ColScheduleConfig), expression.Value(notification.Schedule))
	}

	update = update.Set(expression.Name(ColScheduleUpdatedAt), expression.Value(shared.GetCurrentTime()))

	out, err := services.DbUpdateItem(ctx, services.DbUpdateItemInput{
		TableName: shared.SchedulesTable,
		Update:    update,
		Query: shared.ScheduledNotification{
			ScheduleID: notification.ScheduleID,
		},
		Condition: expression.Name(ColScheduleID).Equal(expression.Value(notification.ScheduleID)),
	})
	if err != nil {
		return shared.ScheduledNotification{}, err
	}

	var updatedNotification shared.ScheduledNotification
	err = attributevalue.UnmarshalMap(out.Attributes, &updatedNotification)
	if err != nil {
		return shared.ScheduledNotification{}, err
	}

	return updatedNotification, nil
}

func DeleteScheduledNotification(ctx context.Context, scheduleID string) error {
	return services.DbDeleteItem(ctx, shared.SchedulesTable, shared.ScheduledNotification{
		ScheduleID: scheduleID,
	})
}

func GetScheduledNotificationsList(ctx context.Context, limit int, startKey string) ([]shared.ScheduledNotification, string, error) {
	var lastEvaluatedKey map[string]types.AttributeValue
	var err error
	if startKey != "" {
		lastEvaluatedKey, err = attributevalue.MarshalMap(map[string]any{
			ColScheduleID: startKey,
		})
		if err != nil {
			return nil, "", err
		}
	}

	var items []shared.ScheduledNotification
	lastEvaluatedKey, err = services.DbScanItems(ctx, shared.SchedulesTable, nil, nil, lastEvaluatedKey, limit, &items)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if lastEvaluatedKey != nil && lastEvaluatedKey[ColScheduleID] != nil {
		nextToken = lastEvaluatedKey[ColScheduleID].(*types.AttributeValueMemberS).Value
	}

	return items, nextToken, nil
}

// GetActiveSchedulesCount gets count of active scheduled notifications for monitoring
func GetActiveSchedulesCount(ctx context.Context) (int, error) {
	filter := expression.Name(ColScheduleStatus).Equal(expression.Value(shared.StatusActive))

	var items []shared.ScheduledNotification
	_, err := services.DbScanItems(ctx, shared.SchedulesTable, &filter, nil, nil, 1000, &items) // Large limit for counting
	if err != nil {
		return 0, err
	}

	return len(items), nil
}
