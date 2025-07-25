package main

import (
	"context"
	"net/http"
	"notification-service/functions/db"
	"notification-service/functions/shared"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	LimitQueryParam     = "limit"
	NextTokenQueryParam = "nextToken"
	ContextQueryParam   = "context"
)

func init() {
	shared.InitAWS()
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (shared.APIResponse, error) {
	shared.LogInfo().Str("method", event.HTTPMethod).Str("path", event.Path).Msg("Config handler invoked")

	// Extract user info from context
	userContext, err := shared.GetUserContext(event.RequestContext)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get user ID from context")
		return shared.CreateErrorResponse(http.StatusUnauthorized, "Invalid authentication", nil), nil
	}

	switch event.HTTPMethod {
	case http.MethodPost:
		return createSystemConfig(ctx, event, userContext)
	case http.MethodPut:
		return updateSystemConfig(ctx, event, userContext)
	case http.MethodGet:
		// Check if this is a request for a specific config (has context query parameter)
		if event.QueryStringParameters != nil && event.QueryStringParameters[ContextQueryParam] != "" {
			return getSystemConfig(ctx, event, userContext)
		}
		return listSystemConfigs(ctx, event, userContext)
	case http.MethodDelete:
		return deleteSystemConfig(ctx, event, userContext)
	default:
		return shared.CreateErrorResponse(http.StatusMethodNotAllowed, "Method not allowed", nil), nil
	}
}

type SystemConfigRequest struct {
	Context     string                `json:"context"`
	Config      shared.SystemSettings `json:"config,omitempty"`
	Description string                `json:"description,omitempty"`
}

func validateUserConfigPermissions(config shared.SystemSettings, context string) shared.APIResponse {
	// Users can only modify specific fields
	if context != "*" {
		// Check if user is trying to modify forbidden fields
		if config.EmailSettings.FromAddress != "" || config.EmailSettings.ReplyToAddress != "" {
			return shared.CreateErrorResponse(http.StatusForbidden, "Users cannot modify email addresses", nil)
		}
	} else {
		// Super admins
		if config.SlackSettings.WebhookURL != "" || len(config.InAppSettings.PlatformAppIDs) != 0 {
			return shared.CreateErrorResponse(http.StatusForbidden, "Super admins cannot modify slack webhook url or in app platform app ids", nil)
		}
	}
	return shared.APIResponse{}
}

func createSystemConfig(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	var request SystemConfigRequest
	err := shared.ParseRequestBody(event.Body, &request)
	if err != nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	context, errResponse := shared.ValidateContext(request.Context, userContext)
	if context == "" {
		return errResponse, nil
	}
	request.Context = context

	// Cannot compare struct with slices directly; check if all config fields are empty
	isSlackEmpty := request.Config.SlackSettings == (shared.SlackSettings{})
	isEmailEmpty := request.Config.EmailSettings == (shared.EmailSettings{})
	isInAppEmpty := request.Config.InAppSettings.Enabled == nil && len(request.Config.InAppSettings.PlatformAppIDs) == 0

	if isSlackEmpty && isEmailEmpty && isInAppEmpty {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Config is required", nil), nil
	}

	// Validate user permissions for config fields
	if errResponse := validateUserConfigPermissions(request.Config, context); errResponse.StatusCode != 0 {
		return errResponse, nil
	}

	// Check if config already exists
	existing, err := db.GetSystemConfig(ctx, request.Context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to check existing config")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to check existing config", nil), nil
	}
	if existing.Context != "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "System config already exists", nil), nil
	}

	// Create new system config
	systemConfig := shared.SystemConfig{
		Context:     request.Context,
		Config:      &request.Config,
		Description: request.Description,
	}

	err = db.CreateSystemConfig(ctx, systemConfig)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to create system config")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to create system config", nil), nil
	}

	shared.LogInfo().Str("context", systemConfig.Context).Msg("System config created successfully")

	return shared.CreateAPIResponse(http.StatusCreated, systemConfig), nil
}

func updateSystemConfig(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	var request SystemConfigRequest
	err := shared.ParseRequestBody(event.Body, &request)
	if err != nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	context, errResponse := shared.ValidateContext(request.Context, userContext)
	if context == "" {
		return errResponse, nil
	}
	request.Context = context

	// Cannot compare struct with slices directly; check if all config fields are empty
	isSlackEmpty := request.Config.SlackSettings == (shared.SlackSettings{})
	isEmailEmpty := request.Config.EmailSettings == (shared.EmailSettings{})
	isInAppEmpty := request.Config.InAppSettings.Enabled == nil && len(request.Config.InAppSettings.PlatformAppIDs) == 0

	if isSlackEmpty && isEmailEmpty && isInAppEmpty && request.Description == "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "At least one field must be provided for update, config or description", nil), nil
	}

	// Get existing config to verify it exists
	existing, err := db.GetSystemConfig(ctx, request.Context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get existing config")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve config", nil), nil
	}
	if existing.Context == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "System config not found", nil), nil
	}

	// For users, merge with existing config to preserve global settings
	if context != "*" {
		// Preserve existing config and only update allowed fields
		mergedConfig := *existing.Config

		// Users can update these fields
		if request.Config.SlackSettings.WebhookURL != "" {
			mergedConfig.SlackSettings.WebhookURL = request.Config.SlackSettings.WebhookURL
		}
		if request.Config.SlackSettings.Enabled != nil {
			mergedConfig.SlackSettings.Enabled = request.Config.SlackSettings.Enabled
		}
		if request.Config.EmailSettings.Enabled != nil {
			mergedConfig.EmailSettings.Enabled = request.Config.EmailSettings.Enabled
		}
		if len(request.Config.InAppSettings.PlatformAppIDs) > 0 {
			mergedConfig.InAppSettings.PlatformAppIDs = request.Config.InAppSettings.PlatformAppIDs
		}
		if request.Config.InAppSettings.Enabled != nil {
			mergedConfig.InAppSettings.Enabled = request.Config.InAppSettings.Enabled
		}

		request.Config = mergedConfig
	}
	// Else we replace the whole config with the new one provided by super admin for global config

	// Validate user permissions for config fields
	if errResponse := validateUserConfigPermissions(request.Config, context); errResponse.StatusCode != 0 {
		return errResponse, nil
	}

	updatedConfig, err := db.UpdateSystemConfig(ctx, shared.SystemConfig{
		Context:     request.Context,
		Config:      &request.Config,
		Description: request.Description,
	})
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to update system config")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to update system config", nil), nil
	}

	shared.LogInfo().Str("context", request.Context).Msg("System config updated successfully")

	return shared.CreateAPIResponse(http.StatusOK, updatedConfig), nil
}

func getSystemConfig(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	context, errResponse := shared.ValidateContext(event.QueryStringParameters[ContextQueryParam], userContext)
	if context == "" {
		return errResponse, nil
	}

	config, err := db.GetSystemConfig(ctx, context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get system config")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve system config", nil), nil
	}

	if config.Context == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "System config not found", nil), nil
	}

	return shared.CreateAPIResponse(http.StatusOK, config), nil
}

func listSystemConfigs(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	// Only super admins can list all configs
	if userContext.Role != shared.RoleSuperAdmin {
		return shared.CreateErrorResponse(http.StatusForbidden, "Only super admins can list all configs", nil), nil
	}

	// Parse query parameters
	limit := shared.GetLimit(event.QueryStringParameters[LimitQueryParam])

	// Handle pagination
	var startKey string
	if nextToken, ok := event.QueryStringParameters[NextTokenQueryParam]; ok && nextToken != "" {
		startKey = nextToken
	}

	// Get configs list
	configs, nextKey, err := db.GetSystemConfigList(ctx, limit, startKey)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get system configs list")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve configs list", nil), nil
	}

	// Create response
	response := shared.PaginatedResponse{
		Items:     configs,
		Count:     len(configs),
		NextToken: nextKey,
	}

	return shared.CreateAPIResponse(http.StatusOK, response), nil
}

func deleteSystemConfig(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	context, errResponse := shared.ValidateContext(event.QueryStringParameters[ContextQueryParam], userContext)
	if context == "" {
		return errResponse, nil
	}

	// Check if config exists before deleting
	existing, err := db.GetSystemConfig(ctx, context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to check existing config")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to check existing config", nil), nil
	}
	if existing.Context == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "System config not found", nil), nil
	}

	err = db.DeleteSystemConfig(ctx, context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to delete system config")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to delete system config", nil), nil
	}

	shared.LogInfo().Str("context", context).Msg("System config deleted successfully")

	return shared.CreateAPIResponse(http.StatusOK, shared.SuccessResponse{Message: "System config deleted successfully"}), nil
}

func main() {
	lambda.Start(handler)
}
