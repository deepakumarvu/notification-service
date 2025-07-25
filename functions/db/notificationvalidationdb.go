package db

import (
	"context"
	"notification-service/functions/services"
	"notification-service/functions/shared"
)

var (
	ColValidationIDUserIDTypeChannel = "id#userId#type#channel"
	ColValidationRecipientID         = "recipientId"
	ColValidationContent             = "content"
	ColValidationCreatedAt           = "createdAt"
	ColValidationError               = "error"
	ColValidationExpiresAt           = "expiresAt"
)

func CreateNotificationValidation(ctx context.Context, validation shared.NotificationValidation) error {
	now := shared.GetCurrentTime()
	validation.CreatedAt = &now

	// Set TTL (1 day from now)
	validation.ExpiresAt = int(now.AddDate(0, 0, 1).Unix())

	return services.DbPutItem(ctx, shared.NotificationValidationTable, validation)
}

func GetNotificationValidation(ctx context.Context, idUserIDTypeChannel string) (shared.NotificationValidation, error) {
	var validation shared.NotificationValidation
	err := services.DbGetItem(ctx, shared.NotificationValidationTable, shared.NotificationValidation{
		IDUserIDTypeChannel: idUserIDTypeChannel,
	}, &validation)
	if err != nil {
		return shared.NotificationValidation{}, err
	}
	return validation, nil
}

func DeleteNotificationValidation(ctx context.Context, idUserIDTypeChannel string) error {
	return services.DbDeleteItem(ctx, shared.NotificationValidationTable, shared.NotificationValidation{
		IDUserIDTypeChannel: idUserIDTypeChannel,
	})
}
