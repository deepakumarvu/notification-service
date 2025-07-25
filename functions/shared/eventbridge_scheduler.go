package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	"github.com/aws/aws-sdk-go-v2/service/scheduler/types"
)

// CreateEventBridgeSchedule creates a new EventBridge Schedule that sends directly to SQS
func CreateEventBridgeSchedule(ctx context.Context, scheduleID, cronExpression string, notificationRequest NotificationRequest) error {
	scheduleName := fmt.Sprintf("schedule-%s", scheduleID)

	// Marshal the complete notification request
	inputJSON, err := json.Marshal(notificationRequest)
	if err != nil {
		LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to marshal notification request")
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	// Create the schedule targeting SQS directly
	_, err = SchedulerClient.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
		Name:                       aws.String(scheduleName),
		Description:                aws.String(fmt.Sprintf("Scheduled notification for %s", scheduleID)),
		ScheduleExpression:         aws.String(fmt.Sprintf("cron(%s)", cronExpression)),
		ScheduleExpressionTimezone: aws.String("UTC"),
		State:                      types.ScheduleStateEnabled,
		FlexibleTimeWindow: &types.FlexibleTimeWindow{
			Mode: types.FlexibleTimeWindowModeOff,
		},
		Target: &types.Target{
			Arn:     aws.String(NotificationQueueArn), // Direct to SQS (ARN format)
			RoleArn: aws.String(SchedulerRoleArn),     // IAM role for EventBridge Scheduler
			Input:   aws.String(string(inputJSON)),
			// No SqsParameters needed for standard SQS queue
		},
	})

	if err != nil {
		LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to create EventBridge schedule")
		return fmt.Errorf("failed to create EventBridge schedule: %w", err)
	}

	LogInfo().Str("scheduleID", scheduleID).Str("scheduleName", scheduleName).Msg("EventBridge schedule created successfully (direct to SQS)")
	return nil
}

// UpdateEventBridgeSchedule updates an existing EventBridge Schedule
func UpdateEventBridgeSchedule(ctx context.Context, scheduleID, cronExpression string, notificationRequest NotificationRequest) error {
	scheduleName := fmt.Sprintf("schedule-%s", scheduleID)

	// Marshal the complete notification request
	inputJSON, err := json.Marshal(notificationRequest)
	if err != nil {
		LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to marshal notification request")
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	// Update the schedule
	_, err = SchedulerClient.UpdateSchedule(ctx, &scheduler.UpdateScheduleInput{
		Name:                       aws.String(scheduleName),
		Description:                aws.String(fmt.Sprintf("Scheduled notification for %s", scheduleID)),
		ScheduleExpression:         aws.String(fmt.Sprintf("cron(%s)", cronExpression)),
		ScheduleExpressionTimezone: aws.String("UTC"),
		State:                      types.ScheduleStateEnabled,
		FlexibleTimeWindow: &types.FlexibleTimeWindow{
			Mode: types.FlexibleTimeWindowModeOff,
		},
		Target: &types.Target{
			Arn:     aws.String(NotificationQueueArn), // Direct to SQS (ARN format)
			RoleArn: aws.String(SchedulerRoleArn),
			Input:   aws.String(string(inputJSON)),
			// No SqsParameters needed for standard SQS queue
		},
	})

	if err != nil {
		LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to update EventBridge schedule")
		return fmt.Errorf("failed to update EventBridge schedule: %w", err)
	}

	LogInfo().Str("scheduleID", scheduleID).Msg("EventBridge schedule updated successfully")
	return nil
}

// DeleteEventBridgeSchedule deletes an EventBridge Schedule
func DeleteEventBridgeSchedule(ctx context.Context, scheduleID string) error {
	scheduleName := fmt.Sprintf("schedule-%s", scheduleID)

	_, err := SchedulerClient.DeleteSchedule(ctx, &scheduler.DeleteScheduleInput{
		Name: aws.String(scheduleName),
	})

	if err != nil {
		LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to delete EventBridge schedule")
		return fmt.Errorf("failed to delete EventBridge schedule: %w", err)
	}

	LogInfo().Str("scheduleID", scheduleID).Msg("EventBridge schedule deleted successfully")
	return nil
}

// PauseEventBridgeSchedule pauses an EventBridge Schedule
func PauseEventBridgeSchedule(ctx context.Context, scheduleID string) error {
	scheduleName := fmt.Sprintf("schedule-%s", scheduleID)

	// Get current schedule details
	getOutput, err := SchedulerClient.GetSchedule(ctx, &scheduler.GetScheduleInput{
		Name: aws.String(scheduleName),
	})
	if err != nil {
		return fmt.Errorf("failed to get schedule details: %w", err)
	}

	// Update with disabled state
	_, err = SchedulerClient.UpdateSchedule(ctx, &scheduler.UpdateScheduleInput{
		Name:                       aws.String(scheduleName),
		Description:                getOutput.Description,
		ScheduleExpression:         getOutput.ScheduleExpression,
		ScheduleExpressionTimezone: getOutput.ScheduleExpressionTimezone,
		State:                      types.ScheduleStateDisabled,
		FlexibleTimeWindow:         getOutput.FlexibleTimeWindow,
		Target:                     getOutput.Target,
	})

	if err != nil {
		LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to pause EventBridge schedule")
		return fmt.Errorf("failed to pause EventBridge schedule: %w", err)
	}

	LogInfo().Str("scheduleID", scheduleID).Msg("EventBridge schedule paused successfully")
	return nil
}

// ResumeEventBridgeSchedule resumes a paused EventBridge Schedule
func ResumeEventBridgeSchedule(ctx context.Context, scheduleID string) error {
	scheduleName := fmt.Sprintf("schedule-%s", scheduleID)

	// Get current schedule details
	getOutput, err := SchedulerClient.GetSchedule(ctx, &scheduler.GetScheduleInput{
		Name: aws.String(scheduleName),
	})
	if err != nil {
		return fmt.Errorf("failed to get schedule details: %w", err)
	}

	// Update with enabled state
	_, err = SchedulerClient.UpdateSchedule(ctx, &scheduler.UpdateScheduleInput{
		Name:                       aws.String(scheduleName),
		Description:                getOutput.Description,
		ScheduleExpression:         getOutput.ScheduleExpression,
		ScheduleExpressionTimezone: getOutput.ScheduleExpressionTimezone,
		State:                      types.ScheduleStateEnabled,
		FlexibleTimeWindow:         getOutput.FlexibleTimeWindow,
		Target:                     getOutput.Target,
	})

	if err != nil {
		LogError().Err(err).Str("scheduleID", scheduleID).Msg("Failed to resume EventBridge schedule")
		return fmt.Errorf("failed to resume EventBridge schedule: %w", err)
	}

	LogInfo().Str("scheduleID", scheduleID).Msg("EventBridge schedule resumed successfully")
	return nil
}

// ValidateCronExpression validates a cron expression for EventBridge Scheduler
// EventBridge Scheduler requires 6-field cron format: minute hour day-of-month month day-of-week year
// IMPORTANT: Cannot use '*' in both day-of-month and day-of-week. Use '?' in one if '*' in the other.
// Examples:
//
//	"0 9 * * ? *" (daily at 9 AM)
//	"0 9 ? * MON *" (every Monday at 9 AM)
//	"0 9 15 * ? *" (15th of every month at 9 AM)
func ValidateCronExpression(cronExpr string) error {
	if cronExpr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	// Basic field count validation for EventBridge Scheduler (6 fields required)
	fields := strings.Fields(cronExpr)
	if len(fields) != 6 {
		return fmt.Errorf("cron expression must have 6 fields (minute hour day-of-month month day-of-week year), got %d fields", len(fields))
	}

	// Validate the day-of-month and day-of-week constraint
	dayOfMonth := fields[2] // 3rd field
	dayOfWeek := fields[4]  // 5th field

	if dayOfMonth == "*" && dayOfWeek == "*" {
		return fmt.Errorf("cannot use '*' in both day-of-month and day-of-week fields. Use '?' in one of them")
	}

	// Let EventBridge validate the detailed syntax
	return nil
}
