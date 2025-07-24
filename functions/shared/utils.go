package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// AWS service clients
var (
	DynamoDBClient    *dynamodb.Client
	SQSClient         *sqs.Client
	SNSClient         *sns.Client
	SESClient         *ses.Client
	EventBridgeClient *eventbridge.Client
	AWSConfig         aws.Config
)

// Environment variables
var (
	UsersTable  string
	UserPoolID  string
	Environment string
	Region      string
)

// InitAWS initializes AWS service clients and environment variables
func InitAWS() {
	// Initialize environment variables
	UsersTable = os.Getenv("USERS_TABLE")
	UserPoolID = os.Getenv("USER_POOL_ID")
	Environment = os.Getenv("ENVIRONMENT")
	Region = os.Getenv("REGION")

	// Load AWS configuration
	var err error
	AWSConfig, err = config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(Region),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Initialize service clients
	DynamoDBClient = dynamodb.NewFromConfig(AWSConfig)
	SQSClient = sqs.NewFromConfig(AWSConfig)
	SNSClient = sns.NewFromConfig(AWSConfig)
	SESClient = ses.NewFromConfig(AWSConfig)
	EventBridgeClient = eventbridge.NewFromConfig(AWSConfig)
}

// CreateAPIResponse creates a standard API Gateway response
func CreateAPIResponse(statusCode int, body interface{}) APIResponse {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		log.Printf("Error marshaling response body: %v", err)
		return CreateErrorResponse(http.StatusInternalServerError, "Failed to marshal response", nil)
	}

	return APIResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Headers": "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
			"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
		},
		Body: string(bodyJSON),
	}
}

// CreateErrorResponse creates a standard error response
func CreateErrorResponse(statusCode int, message string, details interface{}) APIResponse {
	errorResp := ErrorResponse{
		Message: message,
		Details: details,
	}

	return CreateAPIResponse(statusCode, errorResp)
}

// ParseRequestBody parses the request body into the given struct
func ParseRequestBody(body string, target interface{}) error {
	if body == "" {
		return fmt.Errorf("request body is empty")
	}

	return json.Unmarshal([]byte(body), target)
}

func GetLimit(limitStr string) int {
	limit := 50
	if limitStr != "" {
		limitVal, err := strconv.Atoi(limitStr)
		if err != nil {
			return 0
		}
		limit = limitVal
	}
	return limit
}

// ValidateNotificationType validates if the notification type is valid
func ValidateNotificationType(notificationType string) bool {
	validTypes := []string{NotificationTypeAlert, NotificationTypeReport, NotificationTypeNotification}
	for _, validType := range validTypes {
		if notificationType == validType {
			return true
		}
	}
	return false
}

// ValidateChannel validates if the channel is valid
func ValidateChannel(channel string) bool {
	validChannels := []string{ChannelEmail, ChannelSlack, ChannelInApp}
	for _, validChannel := range validChannels {
		if channel == validChannel {
			return true
		}
	}
	return false
}

// GetCurrentTime returns the current time in UTC
func GetCurrentTime() time.Time {
	return time.Now().UTC()
}

// GetUserIDFromContext extracts user ID from the Lambda context/claims
// This would be populated by the API Gateway Cognito authorizer
func GetUserContext(requestContext events.APIGatewayProxyRequestContext) (UserContext, error) {
	// With Cognito User Pool authorizer, claims are in requestContext.Authorizer
	if requestContext.Authorizer == nil {
		return UserContext{}, fmt.Errorf("authorizer context not found")
	}

	claims, ok := requestContext.Authorizer["claims"].(map[string]interface{})
	if !ok {
		return UserContext{}, fmt.Errorf("claims not found in authorizer context")
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return UserContext{}, fmt.Errorf("user ID (sub) not found in claims")
	}

	email, ok := claims["email"].(string)
	if !ok {
		return UserContext{}, fmt.Errorf("email not found in claims")
	}

	role, ok := claims["custom:role"].(string)
	if !ok {
		return UserContext{}, fmt.Errorf("role not found in claims")
	}

	return UserContext{UserID: userID, Email: email, Role: role}, nil
}

// BuildTypeChannel creates the composite key for templates
func BuildTypeChannel(notificationType, channel string) string {
	return notificationType + "#" + channel
}

// ParseTypeChannel splits the composite key into type and channel
func ParseTypeChannel(typeChannel string) (notificationType, channel string) {
	parts := strings.Split(typeChannel, "#")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func ExtractVariablesFromContent(content string) []string {
	re := regexp.MustCompile(`{{.*?}}`)
	matches := re.FindAllString(content, -1)
	// Trip {} from the matches
	for i, match := range matches {
		matches[i] = strings.Trim(match, "{}")
	}
	return matches
}

// ValidateTemplateFixedVariables validates that the template uses only allowed variables for its type
func ValidateTemplateFixedVariables(notificationType string, providedVars []string) []string {
	// Define allowed variables for each notification type
	allowedVars := map[string][]string{
		"alert":        {"serverName", "environment", "status", "message"},
		"report":       {"reportType", "period", "data"},
		"notification": {"title", "message", "actionUrl"},
	}

	allowed, exists := allowedVars[notificationType]
	if !exists {
		return []string{"unknown notification type"}
	}

	var invalid []string
	for _, provided := range providedVars {
		found := false
		for _, allowed := range allowed {
			if provided == allowed {
				found = true
				break
			}
		}
		if !found {
			invalid = append(invalid, provided)
		}
	}
	return invalid
}
