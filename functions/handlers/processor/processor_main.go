package main

import (
	"context"
	"encoding/json"
	"fmt"
	"notification-service/functions/db"
	"notification-service/functions/shared"
	"regexp"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	shared.InitAWS()
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) (events.SQSEventResponse, error) {
	shared.LogInfo().Int("recordCount", len(sqsEvent.Records)).Msg("Notification processor started")

	var failedRecords []events.SQSBatchItemFailure

	for _, record := range sqsEvent.Records {
		err := processMessage(ctx, record)
		if err != nil {
			shared.LogError().Err(err).Str("messageId", record.MessageId).Msg("Failed to process message")
			// Continue processing other messages even if one fails
			failedRecords = append(failedRecords, events.SQSBatchItemFailure{
				ItemIdentifier: record.MessageId,
			})
			continue
		}

	}

	shared.LogInfo().Msg("Notification processor completed")
	return events.SQSEventResponse{
		BatchItemFailures: failedRecords,
	}, nil
}

func processMessage(ctx context.Context, record events.SQSMessage) error {
	shared.LogInfo().Str("messageId", record.MessageId).Msg("Processing notification message")

	// Parse notification request from SQS message body
	var notificationRequest shared.NotificationRequest
	err := json.Unmarshal([]byte(record.Body), &notificationRequest)
	if err != nil {
		shared.LogError().Err(err).Str("messageId", record.MessageId).Msg("Failed to parse notification request")
		return err
	}

	// Process the notification request
	result, err := ProcessNotificationRequest(ctx, notificationRequest)
	if err != nil {
		shared.LogError().Err(err).Str("messageId", record.MessageId).Msg("Failed to process notification request")
		return err
	}

	// Log processing results
	shared.LogInfo().
		Str("messageId", record.MessageId).
		Str("requestType", notificationRequest.Type).
		Int("totalRecipients", result.TotalRecipients).
		Int("successCount", result.SuccessCount).
		Int("failureCount", result.FailureCount).
		Any("Notifications", result.Notifications).
		Msg("Notification processing completed")

	return nil
}

// ProcessingResult represents the result of processing a notification request
type ProcessingResult struct {
	RequestID       string                  `json:"requestId"`
	TotalRecipients int                     `json:"totalRecipients"`
	SuccessCount    int                     `json:"successCount"`
	FailureCount    int                     `json:"failureCount"`
	Notifications   []ProcessedNotification `json:"notifications"`
}

// ProcessedNotification represents a single processed notification
type ProcessedNotification struct {
	RecipientID string `json:"recipientId"`
	Type        string `json:"type"`
	Channel     string `json:"channel"`
	Content     string `json:"content"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"` // error message if failed
}

// ProcessNotificationRequest processes a notification request for all recipients
func ProcessNotificationRequest(ctx context.Context, request shared.NotificationRequest) (*ProcessingResult, error) {
	shared.LogInfo().
		Str("type", request.Type).
		Int("recipientCount", len(request.Recipients)).
		Msg("Starting notification request processing")

	result := &ProcessingResult{
		RequestID:       request.ID,
		TotalRecipients: len(request.Recipients),
		Notifications:   make([]ProcessedNotification, 0),
	}

	// Process each recipient sequentially
	for _, recipientID := range request.Recipients {
		notifications, err := processRecipient(ctx, recipientID, request)
		if err != nil {
			shared.LogError().Err(err).Str("recipientId", recipientID).Msg("Failed to process recipient")
			result.FailureCount++

			// Add failed notification record
			result.Notifications = append(result.Notifications, ProcessedNotification{
				RecipientID: recipientID,
				Success:     false,
				Error:       err.Error(),
			})

			// Add failed notification record to notification validation
			err = db.CreateNotificationValidation(ctx, shared.NotificationValidation{
				IDUserIDTypeChannel: shared.BuildIDUserIDTypeChannel(request.ID, recipientID, request.Type, ""),
				Content:             "",
				Error:               err.Error(),
			})
			if err != nil {
				shared.LogError().Err(err).Str("recipientId", recipientID).Msg("Failed to create notification validation")
			}
			continue
		}

		// Add successful notifications to notification validation
		for _, notification := range notifications {
			err := db.CreateNotificationValidation(ctx, shared.NotificationValidation{
				IDUserIDTypeChannel: shared.BuildIDUserIDTypeChannel(request.ID, recipientID, request.Type, notification.Channel),
				Content:             notification.Content,
				Error:               notification.Error,
			})
			if err != nil {
				shared.LogError().Err(err).Str("recipientId", recipientID).Msg("Failed to create notification validation")
			}
		}

		// Add successful notifications
		result.Notifications = append(result.Notifications, notifications...)
		result.SuccessCount++
	}

	return result, nil
}

// processRecipient processes notifications for a single recipient
func processRecipient(ctx context.Context, recipientID string, request shared.NotificationRequest) ([]ProcessedNotification, error) {
	shared.LogInfo().Str("recipientId", recipientID).Str("type", request.Type).Msg("Processing recipient")

	// Step 1: Get effective user preferences (user-specific → global fallback)
	preferences, err := getEffectivePreferences(ctx, recipientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective preferences: %w", err)
	}

	// Step 2: Get effective system config (user-specific → global fallback)
	config, err := getEffectiveConfig(ctx, recipientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective config: %w", err)
	}

	// Step 3: Filter enabled channels
	enabledChannels := filterEnabledChannels(preferences, config, request.Type)
	if len(enabledChannels) == 0 {
		shared.LogInfo().Str("recipientId", recipientID).Msg("No enabled channels for recipient")
		return []ProcessedNotification{}, nil
	}

	// Step 4: Process template and create notifications for each enabled channel
	notifications := make([]ProcessedNotification, 0)

	for _, channel := range enabledChannels {
		// Step 5: Get required template (user-specific → global → fatal error)
		template, err := getRequiredTemplate(ctx, recipientID, request.Type, channel)
		if err != nil {
			return nil, fmt.Errorf("failed to get required template: %w", err)
		}
		content, err := processTemplateForChannel(template.Content, channel, request.Variables)
		if err != nil {
			shared.LogError().Err(err).Str("recipientId", recipientID).Str("channel", channel).Msg("Failed to process template")
			notifications = append(notifications, ProcessedNotification{
				RecipientID: recipientID,
				Type:        request.Type,
				Channel:     channel,
				Success:     false,
				Error:       err.Error(),
			})
			continue
		}

		notifications = append(notifications, ProcessedNotification{
			RecipientID: recipientID,
			Channel:     channel,
			Content:     content,
			Success:     true,
		})
	}

	return notifications, nil
}

// getEffectivePreferences gets user preferences with global fallback
func getEffectivePreferences(ctx context.Context, recipientID string) (shared.UserPreferences, error) {
	// Try user-specific preferences first
	userPrefs, err := db.GetUserPreferences(ctx, recipientID)
	if err == nil && userPrefs.Context != "" {
		shared.LogInfo().Str("recipientId", recipientID).Msg("Using user-specific preferences")
		return userPrefs, nil
	}

	// Fallback to global preferences
	globalPrefs, err := db.GetUserPreferences(ctx, "*")
	if err == nil && globalPrefs.Context != "" {
		shared.LogInfo().Str("recipientId", recipientID).Msg("Using global preferences fallback")
		return globalPrefs, nil
	}

	// Return error if neither exists
	return shared.UserPreferences{}, fmt.Errorf("no preferences found for recipient %s", recipientID)
}

// getEffectiveConfig gets system config with global fallback
func getEffectiveConfig(ctx context.Context, recipientID string) (shared.SystemConfig, error) {
	// Try user-specific config first
	userConfig, err := db.GetSystemConfig(ctx, recipientID)
	if err == nil && userConfig.Context != "" {
		shared.LogInfo().Str("recipientId", recipientID).Msg("Using user-specific config")
		return userConfig, nil
	}

	// Fallback to global config
	globalConfig, err := db.GetSystemConfig(ctx, "*")
	if err == nil && globalConfig.Context != "" {
		shared.LogInfo().Str("recipientId", recipientID).Msg("Using global config fallback")
		return globalConfig, nil
	}

	// Return error if neither exists
	return shared.SystemConfig{}, fmt.Errorf("no config found for recipient %s", recipientID)
}

// getRequiredTemplate gets template with user → global fallback, error if none found
func getRequiredTemplate(ctx context.Context, recipientID, notificationType, channel string) (shared.Template, error) {
	// Try user-specific template first
	userTemplate, err := db.GetTemplateByTypeChannel(ctx, recipientID, shared.BuildTypeChannel(notificationType, channel))
	if err == nil && userTemplate.Context != "" {
		shared.LogInfo().Str("recipientId", recipientID).Str("type", notificationType).Msg("Using user-specific template")
		return userTemplate, nil
	}

	// Fallback to global template
	globalTemplate, err := db.GetTemplateByTypeChannel(ctx, "*", shared.BuildTypeChannel(notificationType, channel))
	if err == nil && globalTemplate.Context != "" {
		shared.LogInfo().Str("recipientId", recipientID).Str("type", notificationType).Msg("Using global template fallback")
		return globalTemplate, nil
	}

	// Fatal error if no template found
	return shared.Template{}, fmt.Errorf("no template found for type %s (fatal error)", notificationType)
}

// filterEnabledChannels filters channels based on preferences, config, and template availability
func filterEnabledChannels(preferences shared.UserPreferences, config shared.SystemConfig, notificationType string) []string {
	enabledChannels := make([]string, 0)

	// Get preference for this notification type
	prefItem, hasPref := preferences.Preferences[notificationType]
	if !hasPref || prefItem.Enabled == nil || !*prefItem.Enabled {
		shared.LogInfo().Str("type", notificationType).Msg("Notification type disabled in preferences")
		return enabledChannels
	}

	// Check each preferred channel
	for _, channel := range prefItem.Channels {
		// Check if channel is enabled in system config
		if !isChannelEnabledInConfig(config, channel) {
			shared.LogInfo().Str("channel", channel).Msg("Channel disabled in system config")
			continue
		}

		enabledChannels = append(enabledChannels, channel)
	}

	return enabledChannels
}

// isChannelEnabledInConfig checks if a channel is enabled in system config
func isChannelEnabledInConfig(config shared.SystemConfig, channel string) bool {
	if config.Config == nil {
		return false
	}

	switch channel {
	case shared.ChannelEmail:
		return config.Config.EmailSettings.Enabled != nil && *config.Config.EmailSettings.Enabled
	case shared.ChannelSlack:
		return config.Config.SlackSettings.Enabled != nil && *config.Config.SlackSettings.Enabled
	case shared.ChannelInApp:
		return config.Config.InAppSettings.Enabled != nil && *config.Config.InAppSettings.Enabled
	default:
		return false
	}
}

// processTemplateForChannel processes template variables for a specific channel
func processTemplateForChannel(templateContent, channel string, variables map[string]any) (string, error) {
	if templateContent == "" {
		return "", fmt.Errorf("template content is empty")
	}

	shared.LogInfo().Str("channel", channel).Msg("Processing template for channel")

	// Parse template content based on channel
	var processedContent string
	var err error

	switch channel {
	case shared.ChannelEmail:
		processedContent, err = processEmailTemplate(templateContent, variables)
	case shared.ChannelSlack:
		processedContent, err = processSlackTemplate(templateContent, variables)
	case shared.ChannelInApp:
		processedContent, err = processInAppTemplate(templateContent, variables)
	default:
		return "", fmt.Errorf("unsupported channel: %s", channel)
	}

	if err != nil {
		return "", fmt.Errorf("failed to process template for channel %s: %w", channel, err)
	}

	return processedContent, nil
}

// processEmailTemplate processes email template with subject and body
func processEmailTemplate(templateContent string, variables map[string]any) (string, error) {
	// Email templates are expected to be JSON with subject and body
	var emailTemplate map[string]string
	err := json.Unmarshal([]byte(templateContent), &emailTemplate)
	if err != nil {
		return "", fmt.Errorf("invalid email template format: %w", err)
	}

	subject, hasSubject := emailTemplate["subject"]
	body, hasBody := emailTemplate["body"]

	if !hasSubject || !hasBody {
		return "", fmt.Errorf("email template must have both subject and body")
	}

	// Process variables in subject and body
	processedSubject := replaceTemplateVariables(subject, variables)
	processedBody := replaceTemplateVariables(body, variables)

	// Return as JSON
	result := map[string]string{
		"subject": processedSubject,
		"body":    processedBody,
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal processed email template: %w", err)
	}

	return string(resultBytes), nil
}

// processSlackTemplate processes Slack template (simple text with variables)
func processSlackTemplate(templateContent string, variables map[string]any) (string, error) {
	// Slack templates can be simple text or JSON with more complex formatting
	// For now, treat as simple text with variable replacement
	return replaceTemplateVariables(templateContent, variables), nil
}

// processInAppTemplate processes in-app template (simple text with variables)
func processInAppTemplate(templateContent string, variables map[string]any) (string, error) {
	// In-app templates can be simple text or JSON with more complex formatting
	// For now, treat as simple text with variable replacement
	return replaceTemplateVariables(templateContent, variables), nil
}

// replaceTemplateVariables replaces template variables in the format {{variableName}}
func replaceTemplateVariables(content string, variables map[string]any) string {
	// Pattern to match {{variableName}}
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name (remove {{ and }})
		varName := strings.Trim(match, "{}")
		varName = strings.TrimSpace(varName)

		// Look up variable value
		if value, exists := variables[varName]; exists {
			return fmt.Sprintf("%v", value)
		}

		// Replace missing variables with empty string as per requirements
		shared.LogInfo().Str("variable", varName).Msg("Template variable not found, replacing with empty string")
		return ""
	})
}

func main() {
	lambda.Start(handler)
}
