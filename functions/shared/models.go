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

// UserPreferences represents user notification preferences
type UserPreferences struct {
	Context     string                    `json:"context" dynamodbav:"context"` // "*" for global, userId for user-specific
	Preferences map[string]PreferenceItem `json:"preferences,omitempty" dynamodbav:"preferences,omitempty"`
	Timezone    string                    `json:"timezone,omitempty" dynamodbav:"timezone,omitempty"`
	Language    string                    `json:"language,omitempty" dynamodbav:"language,omitempty"`
	CreatedAt   *time.Time                `json:"createdAt,omitempty" dynamodbav:"createdAt,omitempty"`
	UpdatedAt   *time.Time                `json:"updatedAt,omitempty" dynamodbav:"updatedAt,omitempty"`
}

// PreferenceItem represents preferences for a notification type
type PreferenceItem struct {
	Channels []string `json:"channels,omitempty" dynamodbav:"channels,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty" dynamodbav:"enabled,omitempty"`
}

// ScheduledNotification represents a scheduled notification
type ScheduledNotification struct {
	ScheduleID string          `json:"scheduleId,omitempty" dynamodbav:"scheduleId,omitempty"`
	UserID     string          `json:"userId,omitempty" dynamodbav:"userId,omitempty"`
	Type       string          `json:"type,omitempty" dynamodbav:"type,omitempty"`
	Variables  map[string]any  `json:"variables,omitempty" dynamodbav:"variables,omitempty"`
	Schedule   *ScheduleConfig `json:"schedule,omitempty" dynamodbav:"schedule,omitempty"`
	Status     string          `json:"status,omitempty" dynamodbav:"status,omitempty"` // "active" | "paused" | "cancelled"
	CreatedAt  *time.Time      `json:"createdAt,omitempty" dynamodbav:"createdAt,omitempty"`
	UpdatedAt  *time.Time      `json:"updatedAt,omitempty" dynamodbav:"updatedAt,omitempty"`
}

// ScheduleConfig represents the scheduling configuration
type ScheduleConfig struct {
	Type       string `json:"type,omitempty" dynamodbav:"type,omitempty"`             // "one_time" | "recurring" | "cron"
	Expression string `json:"expression,omitempty" dynamodbav:"expression,omitempty"` // ISO timestamp or cron expression
}

// SystemConfig represents system configuration
type SystemConfig struct {
	Context     string          `json:"context,omitempty" dynamodbav:"context,omitempty"` // "*" for global, userId for user-specific
	Config      *SystemSettings `json:"config,omitempty" dynamodbav:"config,omitempty"`   // The actual configuration object
	Description string          `json:"description,omitempty" dynamodbav:"description,omitempty"`
	CreatedAt   *time.Time      `json:"createdAt,omitempty" dynamodbav:"createdAt,omitempty"`
	UpdatedAt   *time.Time      `json:"updatedAt,omitempty" dynamodbav:"updatedAt,omitempty"`
}

// SystemSettings represents the actual system settings data
type SystemSettings struct {
	SlackSettings SlackSettings `json:"slack,omitempty" dynamodbav:"slack,omitempty"`
	EmailSettings EmailSettings `json:"email,omitempty" dynamodbav:"email,omitempty"`
	InAppSettings InAppSettings `json:"inApp,omitempty" dynamodbav:"inApp,omitempty"`
}

// SlackSettings represents Slack configuration
type SlackSettings struct {
	WebhookURL string `json:"webhookUrl,omitempty" dynamodbav:"webhookUrl,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty" dynamodbav:"enabled,omitempty"`
}

// EmailSettings represents email configuration
type EmailSettings struct {
	FromAddress    string `json:"fromAddress,omitempty" dynamodbav:"fromAddress,omitempty"`
	ReplyToAddress string `json:"replyToAddress,omitempty" dynamodbav:"replyToAddress,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty" dynamodbav:"enabled,omitempty"`
}

// InAppSettings represents in-app notification configuration
type InAppSettings struct {
	PlatformAppIDs []string `json:"platformAppIds,omitempty" dynamodbav:"platformAppIds,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty" dynamodbav:"enabled,omitempty"`
}

// NotificationRequest represents a request to send a notification
type NotificationRequest struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Recipients []string       `json:"recipients"`
	Variables  map[string]any `json:"variables"`
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

// NotificationValidation represents a notification validation
type NotificationValidation struct {
	IDUserIDTypeChannel string     `json:"id#userId#type#channel" dynamodbav:"id#userId#type#channel"`
	Content             string     `json:"content,omitempty" dynamodbav:"content,omitempty"`
	CreatedAt           *time.Time `json:"createdAt,omitempty" dynamodbav:"createdAt,omitempty"`
	Error               string     `json:"error,omitempty" dynamodbav:"error,omitempty"`
	ExpiresAt           int        `json:"expiresAt,omitempty" dynamodbav:"expiresAt,omitempty"` // 1 day expiration
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

// Constants for schedule types
const (
	ScheduleTypeCron = "cron"
)

// Constants for notification status
const (
	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusCancelled = "cancelled"
	StatusCompleted = "completed"
)
