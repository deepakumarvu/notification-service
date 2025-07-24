package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"notification-service/functions/db"
	"notification-service/functions/shared"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	TemplateIDPathParam = "templateId"
	LimitQueryParam     = "limit"
	NextTokenQueryParam = "nextToken"
	ContextQueryParam   = "context"
)

func init() {
	shared.InitAWS()
}

func validateTemplateID(templateID string) (string, shared.APIResponse) {
	if templateID == "" {
		return "", shared.CreateErrorResponse(http.StatusBadRequest, "Template ID is required", nil)
	}

	typeChannel, err := url.QueryUnescape(templateID)
	if err != nil {
		return "", shared.CreateErrorResponse(http.StatusBadRequest, "Invalid template ID encoding", nil)
	}

	notificationType, channel := shared.ParseTypeChannel(typeChannel)
	if notificationType == "" || channel == "" {
		return "", shared.CreateErrorResponse(http.StatusBadRequest, "Template ID must be in format 'type#channel'", nil)
	}

	return typeChannel, shared.APIResponse{}
}

func validateContext(context string, userContext shared.UserContext) (string, shared.APIResponse) {

	if context == "*" && userContext.Role != shared.RoleSuperAdmin {
		return "", shared.CreateErrorResponse(http.StatusForbidden, "Global templates are only allowed for super admins", nil)
	}

	if userContext.Role == shared.RoleUser || context == "" {
		context = userContext.UserID
	}

	return context, shared.APIResponse{}
}

func handler(ctx context.Context, event events.APIGatewayProxyRequest) (shared.APIResponse, error) {
	shared.LogInfo().Str("method", event.HTTPMethod).Str("path", event.Path).Msg("Template handler invoked")

	// Extract user info from context
	userContext, err := shared.GetUserContext(event.RequestContext)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get user ID from context")
		return shared.CreateErrorResponse(http.StatusUnauthorized, "Invalid authentication", nil), nil
	}

	switch event.HTTPMethod {
	case http.MethodPost:
		return createTemplate(ctx, event, userContext)
	case http.MethodPut:
		return updateTemplate(ctx, event, userContext)
	case http.MethodGet:
		// Check if this is a request for a specific template (has templateId path parameter)
		if event.PathParameters != nil && event.PathParameters[TemplateIDPathParam] != "" {
			return getTemplateByID(ctx, event, userContext)
		}
		return listTemplates(ctx, event, userContext)
	case http.MethodDelete:
		return deleteTemplate(ctx, event, userContext)
	default:
		return shared.CreateErrorResponse(http.StatusMethodNotAllowed, "Method not allowed", nil), nil
	}
}

type TemplateRequest struct {
	Context string `json:"context"`
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Content string `json:"content"`
	Enable  *bool  `json:"disable"`
}

func createTemplate(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {

	var request TemplateRequest
	err := shared.ParseRequestBody(event.Body, &request)
	if err != nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	context, errResponse := validateContext(request.Context, userContext)
	if context == "" {
		return errResponse, nil
	}
	request.Context = context

	if request.Type == "" || !shared.ValidateNotificationType(request.Type) {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Valid notification type is required", nil), nil
	}

	if request.Channel == "" || !shared.ValidateChannel(request.Channel) {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Valid channel is required", nil), nil
	}

	if request.Content == "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Template content is required", nil), nil
	}

	variables := shared.ExtractVariablesFromContent(request.Content)

	// Validate template variables against fixed set for the type
	if invalidVars := shared.ValidateTemplateFixedVariables(request.Type, variables); len(invalidVars) > 0 {
		return shared.CreateErrorResponse(http.StatusBadRequest, fmt.Sprintf("Invalid variables for type %s: %v", request.Type, invalidVars), nil), nil
	}

	// Check if template already exists
	existing, err := db.GetTemplateByTypeChannel(ctx, request.Context, shared.BuildTypeChannel(request.Type, request.Channel))
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get existing template")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve template", nil), nil
	}
	if existing.TypeChannel != "" {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Template already exists", nil), nil
	}

	// Create new template
	template := shared.Template{
		Context:     request.Context,
		TypeChannel: shared.BuildTypeChannel(request.Type, request.Channel),
		Content:     request.Content,
		IsActive:    &db.TemplateActive,
	}

	err = db.CreateTemplate(ctx, template)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to create template")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to create template", nil), nil
	}

	shared.LogInfo().Str("context", template.Context).Str("typeChannel", template.TypeChannel).Msg("Template created successfully")

	return shared.CreateAPIResponse(http.StatusCreated, template), nil
}

func updateTemplate(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {

	typeChannel, errResponse := validateTemplateID(event.PathParameters[TemplateIDPathParam])
	if typeChannel == "" {
		return errResponse, nil
	}

	var request TemplateRequest
	err := shared.ParseRequestBody(event.Body, &request)
	if err != nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, "Invalid request body", nil), nil
	}

	context, errResponse := validateContext(request.Context, userContext)
	if context == "" {
		return errResponse, nil
	}
	request.Context = context
	request.Type, request.Channel = shared.ParseTypeChannel(typeChannel)

	// Get existing template to verify ownership
	existing, err := db.GetTemplateByTypeChannel(ctx, request.Context, typeChannel)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get existing template")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve template", nil), nil
	}
	if existing.TypeChannel == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "Template not found", nil), nil
	}

	if request.Content == "" && request.Enable == nil {
		return shared.CreateErrorResponse(http.StatusBadRequest, "At least one field must be provided", nil), nil
	}

	// Validate the request
	if request.Content != "" {
		variables := shared.ExtractVariablesFromContent(request.Content)
		// Validate template variables against fixed set for the type
		if invalidVars := shared.ValidateTemplateFixedVariables(request.Type, variables); len(invalidVars) > 0 {
			return shared.CreateErrorResponse(http.StatusBadRequest, fmt.Sprintf("Invalid variables for type %s: %v", request.Type, invalidVars), nil), nil
		}
	}

	updatedTemplate, err := db.UpdateTemplate(ctx, shared.Template{
		Context:     request.Context,
		TypeChannel: typeChannel,
		Content:     request.Content,
		IsActive:    request.Enable,
	})
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to update template")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to update template", nil), nil
	}

	shared.LogInfo().Str("typeChannel", typeChannel).Str("context", existing.Context).Msg("Template updated successfully")

	return shared.CreateAPIResponse(http.StatusOK, updatedTemplate), nil
}

func listTemplates(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {
	context, errResponse := validateContext(event.QueryStringParameters[ContextQueryParam], userContext)
	if context == "" {
		return errResponse, nil
	}

	// Parse query parameters
	limit := shared.GetLimit(event.QueryStringParameters[LimitQueryParam])

	// Handle pagination
	var startKey string
	if nextToken, ok := event.QueryStringParameters[NextTokenQueryParam]; ok && nextToken != "" {
		startKey = nextToken
	}

	// Get templates list
	templates, nextKey, err := db.GetTemplatesList(ctx, context, limit, startKey)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to unmarshal templates")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to process templates", nil), nil
	}

	// Create response
	response := shared.PaginatedResponse{
		Items:     templates,
		Count:     len(templates),
		NextToken: nextKey,
	}

	return shared.CreateAPIResponse(http.StatusOK, response), nil
}

func getTemplateByID(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {

	typeChannel, errResponse := validateTemplateID(event.PathParameters[TemplateIDPathParam])
	if typeChannel == "" {
		return errResponse, nil
	}

	context, errResponse := validateContext(event.QueryStringParameters[ContextQueryParam], userContext)
	if context == "" {
		return errResponse, nil
	}

	template, err := db.GetTemplateByTypeChannel(ctx, context, typeChannel)
	if err != nil {
		shared.LogError().Err(err).Msg("Failed to get template")
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to retrieve template", nil), nil
	}

	if template.Context == "" {
		return shared.CreateErrorResponse(http.StatusNotFound, "Template not found", nil), nil
	}

	return shared.CreateAPIResponse(http.StatusOK, template), nil
}

func deleteTemplate(ctx context.Context, event events.APIGatewayProxyRequest, userContext shared.UserContext) (shared.APIResponse, error) {

	typeChannel, errResponse := validateTemplateID(event.PathParameters[TemplateIDPathParam])
	if typeChannel == "" {
		return errResponse, nil
	}

	context, errResponse := validateContext(event.QueryStringParameters[ContextQueryParam], userContext)
	if context == "" {
		return errResponse, nil
	}

	err := db.DeleteTemplate(ctx, context, typeChannel)
	if err != nil {
		return shared.CreateErrorResponse(http.StatusInternalServerError, "Failed to delete template", nil), nil
	}

	return shared.CreateAPIResponse(http.StatusOK, shared.SuccessResponse{Message: "Template deleted successfully"}), nil

}

func main() {
	lambda.Start(handler)
}
