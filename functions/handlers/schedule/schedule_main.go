package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"notification-service/functions/db"
	"notification-service/functions/shared"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"
)

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (shared.APIResponse, error) {
	shared.InitAWS()

	userContext, err := shared.GetUserContext(request.RequestContext)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get user context")
		return shared.CreateErrorResponse(http.StatusUnauthorized, "Unauthorized", err.Error()), nil
	}

	switch request.HTTPMethod {
	case http.MethodPost:
		return createScheduledNotification(ctx, request, userContext)
	case http.MethodGet:
		if scheduleID := request.PathParameters["scheduleId"]; scheduleID != "" {
			return getScheduledNotification(ctx, scheduleID, userContext)
		}
		return listUserScheduledNotifications(ctx, request, userContext)
	case http.MethodPut:
		scheduleID := request.PathParameters["scheduleId"]
		if scheduleID == "" {
			return shared.CreateErrorResponse(http.StatusBadRequest, "Schedule ID is required", nil), nil
		}
		return updateScheduledNotification(ctx, request, scheduleID, userContext)
	case http.MethodDelete:
		scheduleID := request.PathParameters["scheduleId"]
		if scheduleID == "" {
			return shared.CreateErrorResponse(http.StatusBadRequest, "Schedule ID is required", nil), nil
		}
		return deleteScheduledNotification(ctx, scheduleID, userContext)
	default:
		return shared.CreateErrorResponse(http.StatusMethodNotAllowed, "Method not allowed", nil), nil
	}
}

func createScheduledNotification(ctx context.Context, request events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	var reqBody struct {
		Type      string                `json:"type"`
		Variables map[string]any        `json:"variables"`
		Schedule  shared.ScheduleConfig `json:"schedule"`
	}

	if err := json.Unmarshal([]byte(request.Body), &reqBody); err != nil {
		shared.LogError().Err(err).Msg("Failed to unmarshal request body")
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	// Validate required fields
	if reqBody.Type == "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Type is required", nil), nil
	}
	if reqBody.Schedule.Type != shared.ScheduleTypeCron {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Only cron schedule type is supported", nil), nil
	}
	if reqBody.Schedule.Expression == "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Schedule expression is required", nil), nil
	}

	// Validate cron expression
	if err := shared.ValidateCronExpression(reqBody.Schedule.Expression); err != nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, fmt.Sprintf("Invalid cron expression: %v", err), nil), nil
	}

	// Generate schedule ID
	scheduleID := uuid.New().String()

	// Create notification request payload for direct SQS delivery
	notificationRequest := shared.NotificationRequest{
		ID:         scheduleID,
		Type:       reqBody.Type,
		Recipients: []string{userContext.UserID}, // User is the recipient
		Variables:  reqBody.Variables,
	}

	// Create EventBridge Schedule (direct to SQS)
	if err := shared.CreateEventBridgeSchedule(ctx, scheduleID, reqBody.Schedule.Expression, notificationRequest); err != nil {
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to create EventBridge schedule")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to create schedule", nil), nil
	}

	// Create scheduled notification
	notification := shared.ScheduledNotification{
		ScheduleID: scheduleID,
		UserID:     userContext.UserID,
		Type:       reqBody.Type,
		Variables:  reqBody.Variables,
		Schedule:   &reqBody.Schedule,
		Status:     shared.StatusActive,
	}

	if err := db.CreateScheduledNotification(ctx, notification); err != nil {
		// Clean up EventBridge schedule if database creation fails
		shared.DeleteEventBridgeSchedule(ctx, scheduleID)
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to create scheduled notification")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to create scheduled notification", nil), nil
	}

	shared.LogInfo().Str("scheduleID", scheduleID).Str("userID", userContext.UserID).Msg("Scheduled notification created successfully")

	return shared.CreateAPIResponse(http.StatusCreated, notification), nil
}

func getScheduledNotification(ctx context.Context, scheduleID string, userContext shared.UserContext) (shared.APIResponse, error) {
	notification, err := db.GetScheduledNotification(ctx, scheduleID)
	if err != nil {
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to get scheduled notification")
		return shared.CreateErrorResponse(http.StatusNotFound, "Scheduled notification not found", nil), nil
	}

	// Ensure user can only access their own notifications
	if notification.UserID != userContext.UserID {
		return shared.CreateErrorResponse(http.StatusForbidden, "Access denied", nil), nil
	}

	return shared.CreateAPIResponse(http.StatusOK, notification), nil
}

func listUserScheduledNotifications(ctx context.Context, request events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	limit := 20
	if limitStr := request.QueryStringParameters["limit"]; limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	nextToken := request.QueryStringParameters["nextToken"]

	notifications, nextTokenResult, err := db.GetUserScheduledNotifications(ctx, userContext.UserID, limit, nextToken)
	if err != nil {
		shared.LogError().Err(err).Str("userID", userContext.UserID).Msg("Failed to list user scheduled notifications")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to list scheduled notifications", nil), nil
	}

	response := shared.PaginatedResponse{
		Items:     notifications,
		NextToken: nextTokenResult,
		Count:     len(notifications),
	}

	return shared.CreateAPIResponse(http.StatusOK, response), nil
}

func updateScheduledNotification(ctx context.Context, request events.APIGatewayProxyRequest, scheduleID string, userContext shared.UserContext) (shared.APIResponse, error) {
	// Get existing notification
	existingNotification, err := db.GetScheduledNotification(ctx, scheduleID)
	if err != nil {
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to get existing scheduled notification")
		return shared.CreateErrorResponse(http.StatusNotFound, "Scheduled notification not found", nil), nil
	}

	// Ensure user can only update their own notifications
	if existingNotification.UserID != userContext.UserID {
		return shared.CreateErrorResponse(http.StatusForbidden, "Access denied", nil), nil
	}

	var reqBody struct {
		Variables map[string]any         `json:"variables,omitempty"`
		Schedule  *shared.ScheduleConfig `json:"schedule,omitempty"`
		Status    string                 `json:"status,omitempty"`
	}

	if err := json.Unmarshal([]byte(request.Body), &reqBody); err != nil {
		shared.LogError().Err(err).Msg("Failed to unmarshal request body")
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	updateNotification := shared.ScheduledNotification{
		ScheduleID: scheduleID,
	}

	// Update fields if provided
	if reqBody.Variables != nil {
		updateNotification.Variables = reqBody.Variables
	}
	if reqBody.Status != "" {
		if reqBody.Status != shared.StatusActive && reqBody.Status != shared.StatusPaused && reqBody.Status != shared.StatusCancelled {
			return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid status", nil), nil
		}
		updateNotification.Status = reqBody.Status
	}

	// Handle schedule updates
	if reqBody.Schedule != nil {
		if reqBody.Schedule.Type != shared.ScheduleTypeCron {
			return shared.CreateErrorResponse(http.StatusBadRequest, "Only cron schedule type is supported", nil), nil
		}
		if err := shared.ValidateCronExpression(reqBody.Schedule.Expression); err != nil {
			return shared.CreateErrorResponse(http.StatusBadRequest, fmt.Sprintf("Invalid cron expression: %v", err), nil), nil
		}

		// Create updated notification request payload
		updatedVariables := existingNotification.Variables
		if reqBody.Variables != nil {
			updatedVariables = reqBody.Variables
		}

		updatedNotificationRequest := shared.NotificationRequest{
			ID:         scheduleID,
			Type:       existingNotification.Type,
			Recipients: []string{existingNotification.UserID},
			Variables:  updatedVariables,
		}

		// Update EventBridge schedule
		if err := shared.UpdateEventBridgeSchedule(ctx, scheduleID, reqBody.Schedule.Expression, updatedNotificationRequest); err != nil {
			shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to update EventBridge schedule")
			return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to update schedule", nil), nil
		}

		updateNotification.Schedule = reqBody.Schedule
	}

	// Handle status updates (pause/resume schedules)
	if reqBody.Status != "" {
		if reqBody.Status == shared.StatusPaused {
			if err := shared.PauseEventBridgeSchedule(ctx, scheduleID); err != nil {
				shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to pause EventBridge schedule")
				return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to pause schedule", nil), nil
			}
		} else if reqBody.Status == shared.StatusActive {
			if err := shared.ResumeEventBridgeSchedule(ctx, scheduleID); err != nil {
				shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to resume EventBridge schedule")
				return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to resume schedule", nil), nil
			}
		}
	}

	// Update notification in database
	updatedNotification, err := db.UpdateScheduledNotification(ctx, updateNotification)
	if err != nil {
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to update scheduled notification")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to update scheduled notification", nil), nil
	}

	shared.LogInfo().Str("scheduleID", scheduleID).Str("userID", userContext.UserID).Msg("Scheduled notification updated successfully")

	return shared.CreateAPIResponse(http.StatusOK, updatedNotification), nil
}

func deleteScheduledNotification(ctx context.Context, scheduleID string, userContext shared.UserContext) (shared.APIResponse, error) {
	// Get existing notification
	existingNotification, err := db.GetScheduledNotification(ctx, scheduleID)
	if err != nil {
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to get existing scheduled notification")
		return shared.CreateErrorResponse(http.StatusNotFound, "Scheduled notification not found", nil), nil
	}

	// Ensure user can only delete their own notifications
	if existingNotification.UserID != userContext.UserID {
		return shared.CreateErrorResponse(http.StatusForbidden, "Access denied", nil), nil
	}

	// Delete EventBridge schedule
	if err := shared.DeleteEventBridgeSchedule(ctx, scheduleID); err != nil {
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to delete EventBridge schedule")
		// Continue with deletion even if EventBridge fails
	}

	// Delete from database
	if err := db.DeleteScheduledNotification(ctx, scheduleID); err != nil {
		shared.LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to delete scheduled notification")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to delete scheduled notification", nil), nil
	}

	shared.LogInfo().Str("scheduleID", scheduleID).Str("userID", userContext.UserID).Msg("Scheduled notification deleted successfully")

	return shared.CreateAPIResponse(http.StatusOK, shared.SuccessResponse{Message: "Scheduled notification deleted successfully"}), nil
}
