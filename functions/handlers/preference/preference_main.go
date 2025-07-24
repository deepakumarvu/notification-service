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

func validateContext(context string, userContext shared.UserContext) (string, shared.APIResponse) {
	if context == "*" && userContext.Role != shared.RoleSuperAdmin {
		return "", shared.CreateErrorResponse(http.StatusForbidden, "Global preferences are only allowed for super admins", nil)
	}

	if userContext.Role == shared.RoleUser || context == "" {
		context = userContext.UserID
	}

	return context, shared.APIResponse{}
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (shared.APIResponse, error) {
	shared.LogInfo().Str("method", event.HTTPMethod).Str("path", event.Path).Msg("Preference handler invoked")

	// Extract user info from context
	userContext, err := shared.GetUserContext(event.RequestContext)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get user ID from context")
		return shared.CreateErrorResponse(http.StatusUnauthorized, "Invalid authentication", nil), nil
	}

	switch event.HTTPMethod {
	case http.MethodPost:
		return createUserPreferences(ctx, event, userContext)
	case http.MethodPut:
		return updateUserPreferences(ctx, event, userContext)
	case http.MethodGet:
		// Check if this is a request for a specific user's preferences (has context query parameter)
		if event.QueryStringParameters[ContextQueryParam] != "" {
			return getUserPreferences(ctx, event, userContext)
		}
		return listUserPreferences(ctx, event, userContext)
	case http.MethodDelete:
		return deleteUserPreferences(ctx, event, userContext)
	default:
		return shared.CreateErrorResponse(http.StatusMethodNotAllowed, "Method not allowed", nil), nil
	}
}

type UserPreferencesRequest struct {
	Context     string                           `json:"context"`
	Preferences map[string]shared.PreferenceItem `json:"preferences,omitempty"`
	Timezone    string                           `json:"timezone,omitempty"`
	Language    string                           `json:"language,omitempty"`
}

func createUserPreferences(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	var request UserPreferencesRequest
	err := shared.ParseRequestBody(event.Body, &request)
	if err != nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	context, errResponse := validateContext(request.Context, userContext)
	if context == "" {
		return errResponse, nil
	}
	request.Context = context

	// Validate preferences if provided
	if len(request.Preferences) > 0 {
		for notificationType, prefItem := range request.Preferences {
			if !shared.ValidateNotificationType(notificationType) {
				return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid notification type: "+notificationType, nil), nil
			}
			if prefItem.Channels != nil {
				for _, channel := range prefItem.Channels {
					if !shared.ValidateChannel(channel) {
						return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid channel: "+channel, nil), nil
					}
				}
			}
		}
	} else {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Preferences are required", nil), nil
	}

	// Check if preferences already exist
	existing, err := db.GetUserPreferences(ctx, request.Context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to check existing preferences")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to check existing preferences", nil), nil
	}
	if existing.Context != "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "User preferences already exist", nil), nil
	}

	// Create new user preferences
	userPreferences := shared.UserPreferences{
		Context:     request.Context,
		Preferences: request.Preferences,
		Timezone:    request.Timezone,
		Language:    request.Language,
	}

	err = db.CreateUserPreferences(ctx, userPreferences)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to create user preferences")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to create user preferences", nil), nil
	}

	shared.LogInfo().Str("context", userPreferences.Context).Msg("User preferences created successfully")

	return shared.CreateAPIResponse(http.StatusCreated, userPreferences), nil
}

func updateUserPreferences(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	var request UserPreferencesRequest
	err := shared.ParseRequestBody(event.Body, &request)
	if err != nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	context, errResponse := validateContext(request.Context, userContext)
	if context == "" {
		return errResponse, nil
	}
	request.Context = context

	// Get existing preferences to verify they exist
	existing, err := db.GetUserPreferences(ctx, request.Context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get existing preferences")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve preferences", nil), nil
	}
	if existing.Context == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "User preferences not found", nil), nil
	}

	// Validate at least one field is provided
	if request.Preferences == nil && request.Timezone == "" && request.Language == "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "At least one field must be provided", nil), nil
	}

	// Validate preferences if provided
	if len(request.Preferences) > 0 {
		for notificationType, prefItem := range request.Preferences {
			if !shared.ValidateNotificationType(notificationType) {
				return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid notification type: "+notificationType, nil), nil
			}
			if prefItem.Channels != nil {
				for _, channel := range prefItem.Channels {
					if !shared.ValidateChannel(channel) {
						return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid channel: "+channel, nil), nil
					}
				}
			}
		}
	}

	updatedPreferences, err := db.UpdateUserPreferences(ctx, shared.UserPreferences{
		Context:     request.Context,
		Preferences: request.Preferences,
		Timezone:    request.Timezone,
		Language:    request.Language,
	})
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to update user preferences")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to update user preferences", nil), nil
	}

	shared.LogInfo().Str("context", request.Context).Msg("User preferences updated successfully")

	return shared.CreateAPIResponse(http.StatusOK, updatedPreferences), nil
}

func getUserPreferences(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	context, errResponse := validateContext(event.QueryStringParameters[ContextQueryParam], userContext)
	if context == "" {
		return errResponse, nil
	}

	preferences, err := db.GetUserPreferences(ctx, context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get user preferences")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve user preferences", nil), nil
	}

	if preferences.Context == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "User preferences not found", nil), nil
	}

	return shared.CreateAPIResponse(http.StatusOK, preferences), nil
}

func listUserPreferences(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	// Only super admins can list all preferences
	if userContext.Role != shared.RoleSuperAdmin {
		return shared.CreateErrorResponse(http.StatusForbidden, "Only super admins can list all preferences", nil), nil
	}

	// Parse query parameters
	limit := shared.GetLimit(event.QueryStringParameters[LimitQueryParam])

	// Handle pagination
	var startKey string
	if nextToken, ok := event.QueryStringParameters[NextTokenQueryParam]; ok && nextToken != "" {
		startKey = nextToken
	}

	// Get preferences list
	preferences, nextKey, err := db.GetUserPreferencesList(ctx, limit, startKey)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get user preferences list")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve preferences list", nil), nil
	}

	// Create response
	response := shared.PaginatedResponse{
		Items:     preferences,
		Count:     len(preferences),
		NextToken: nextKey,
	}

	return shared.CreateAPIResponse(http.StatusOK, response), nil
}

func deleteUserPreferences(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	context, errResponse := validateContext(event.QueryStringParameters[ContextQueryParam], userContext)
	if context == "" {
		return errResponse, nil
	}

	// Check if preferences exist before deleting
	existing, err := db.GetUserPreferences(ctx, context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to check existing preferences")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to check existing preferences", nil), nil
	}
	if existing.Context == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "User preferences not found", nil), nil
	}

	err = db.DeleteUserPreferences(ctx, context)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to delete user preferences")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to delete user preferences", nil), nil
	}

	shared.LogInfo().Str("context", context).Msg("User preferences deleted successfully")

	return shared.CreateAPIResponse(http.StatusOK, shared.SuccessResponse{Message: "User preferences deleted successfully"}), nil
}

func main() {
	lambda.Start(handler)
}
