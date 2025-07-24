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
	UserIDPathParam     = "userId"
	LimitQueryParam     = "limit"
	NextTokenQueryParam = "nextToken"
)

func init() {
	shared.InitAWS()
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (shared.APIResponse, error) {
	shared.LogInfo().Str("method", event.HTTPMethod).Str("path", event.Path).Msg("User handler invoked")

	// Extract user info from context
	userContext, err := shared.GetUserContext(event.RequestContext)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get user ID from context")
		return shared.CreateErrorResponse(http.StatusUnauthorized, "Invalid authentication", nil), nil
	}

	switch event.HTTPMethod {
	case http.MethodGet:
		// Check if this is a request for a specific user (has userId path parameter)
		if event.PathParameters != nil && event.PathParameters[UserIDPathParam] != "" {
			return getUserByID(ctx, event, userContext)
		}
		return listUsers(ctx, event, userContext)
	default:
		return shared.CreateErrorResponse(http.StatusMethodNotAllowed, "Method not allowed", nil), nil
	}
}

func listUsers(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	// Only super admin can list all users
	if userContext.Role != shared.RoleSuperAdmin {
		return shared.CreateErrorResponse(http.StatusForbidden, "Insufficient permissions", nil), nil
	}

	// Parse query parameters
	limit := shared.GetLimit(event.QueryStringParameters[LimitQueryParam])

	// Handle pagination
	var startKey string
	if nextToken, ok := event.QueryStringParameters[NextTokenQueryParam]; ok && nextToken != "" {
		startKey = nextToken
	}

	// Get users list
	users, nextKey, err := db.GetUsersList(ctx, limit, startKey)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to unmarshal users")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to process users", nil), nil
	}

	// Create response
	response := shared.PaginatedResponse{
		Items:     users,
		Count:     len(users),
		NextToken: nextKey,
	}

	return shared.CreateAPIResponse(http.StatusOK, response), nil
}

func getUserByID(ctx context.Context, event events.APIGatewayProxyRequest, requestUser shared.UserContext) (shared.APIResponse, error) {
	targetUserID := event.PathParameters[UserIDPathParam]
	if targetUserID == "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "User ID is required", nil), nil
	}

	// Users can only access their own data unless they're super admin
	if requestUser.Role != shared.RoleSuperAdmin && requestUser.UserID != targetUserID {
		return shared.CreateErrorResponse(http.StatusForbidden, "Cannot access other user's data", nil), nil
	}

	user, err := db.GetUserByID(ctx, targetUserID)
	if err != nil {
		shared.LogError().Err(err).Str("userId", targetUserID).Msg("Failed to get user")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve user", nil), nil
	}

	if user == nil {
		return shared.CreateErrorResponse(http.StatusNotFound, "User not found", nil), nil
	}

	return shared.CreateAPIResponse(http.StatusOK, user), nil
}

func main() {
	lambda.Start(handler)
}
