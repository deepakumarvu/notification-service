package shared

import "time"

type UserContext struct {
	UserID string
	Email  string
	Role   string
}

// User represents a user in the notification service
type User struct {
	UserID    string     `json:"userId" dynamodbav:"userId"`
	Email     string     `json:"email,omitempty" dynamodbav:"email,omitempty"`
	Role      string     `json:"role,omitempty" dynamodbav:"role,omitempty"` // "super_admin" | "user"
	IsActive  *bool      `json:"isActive,omitempty" dynamodbav:"isActive,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty" dynamodbav:"createdAt,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty" dynamodbav:"updatedAt,omitempty"`
}

// Template represents a notification template
type Template struct {
	Context     string     `json:"context" dynamodbav:"context"`           // "*" for global, userId for user-specific
	TypeChannel string     `json:"type#channel" dynamodbav:"type#channel"` // "alert#email", "report#slack", etc.
	Content     string     `json:"content,omitempty" dynamodbav:"content,omitempty"`
	IsActive    *bool      `json:"isActive,omitempty" dynamodbav:"isActive,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty" dynamodbav:"createdAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty" dynamodbav:"updatedAt,omitempty"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Message string `json:"message"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Items     any    `json:"items"`
	NextToken string `json:"nextToken,omitempty"`
	Count     int    `json:"count"`
}

// Constants for notification types
const (
	NotificationTypeAlert        = "alert"
	NotificationTypeReport       = "report"
	NotificationTypeNotification = "notification"
)

// Constants for channels
const (
	ChannelEmail = "email"
	ChannelSlack = "slack"
	ChannelInApp = "in_app"
)

// Constants for user roles
const (
	RoleSuperAdmin = "super_admin"
	RoleUser       = "user"
)
