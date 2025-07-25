# System Architecture

## Overview

The Notification Service is designed as a serverless, event-driven system on AWS that provides multi-channel notification delivery with flexible scheduling and templating capabilities.

## Core Principles

1. **Serverless-First**: Leverage AWS Lambda for auto-scaling and cost optimization
2. **Event-Driven**: Use EventBridge and SQS for decoupled, resilient messaging
3. **Multi-Channel**: Support email, Slack, and in-app notifications from a single interface
4. **User-Centric**: Allow users to customize notification preferences and templates
5. **Secure by Design**: Implement proper authentication, authorization, and encryption

## High-Level Architecture

### System Components

#### 1. **API Layer**
- **AWS API Gateway**: RESTful API endpoints
- **AWS Cognito**: User authentication and authorization
- **Cognito Authorizer**: For API Gateway to validate JWT tokens

#### 2. **Business Logic Layer**
- **Lambda Functions**: Go-based functions for core business logic
  - User Management
  - Template Management
  - Notification Processing
  - Scheduling Management
  - Preference Management
  - System Configuration Management

#### 3. **Data Layer**
- **DynamoDB Tables**:
  - Users table
  - Templates table
  - User Preferences table
  - Scheduled Notifications table
  - System Configuration table
  - Notification Validation table (with TTL)

#### 4. **Messaging & Scheduling**
- **Amazon EventBridge Scheduler**: Scheduled notification triggering
- **Amazon SQS**: Message queuing for notification processing
- **Amazon SNS**: In-app push notifications

#### 5. **Delivery Channels**
- **Amazon SES**: Email delivery
- **Slack Webhooks**: Slack message delivery
- **Amazon SNS**: Push notifications for mobile/web apps

#### 6. **Testing & Validation**
- **Notification Validation System**: Tracks delivered notifications for testing
- **Python Test Suite**: Comprehensive API testing framework

## Detailed Component Design

### API Gateway Structure

```
/api/v1/
├── /users/
│   ├── GET /users                     # List all users (super_admin only)
│   └── GET /users/{id}                # Get user by ID
├── /templates/
│   ├── POST /templates                # Create template
│   ├── GET /templates/{context}/{type}/{channel}  # Get specific template
│   ├── PUT /templates/{context}/{type}/{channel}  # Update template
│   └── DELETE /templates/{context}/{type}/{channel}  # Delete template
├── /notifications/
│   ├── POST /send/alert               # Send alert notification
│   ├── POST /send/report              # Send report notification
│   └── POST /send/notification        # Send general notification
├── /scheduled/
│   ├── POST /scheduled                # Create scheduled notification
│   ├── GET /scheduled                 # List user's scheduled notifications
│   ├── GET /scheduled/{id}            # Get specific scheduled notification
│   ├── PUT /scheduled/{id}            # Update scheduled notification
│   ├── PUT /scheduled/{id}/pause      # Pause scheduled notification
│   ├── PUT /scheduled/{id}/resume     # Resume scheduled notification
│   └── DELETE /scheduled/{id}         # Delete scheduled notification
├── /preferences/
│   ├── POST /preferences              # Create user preferences
│   ├── GET /preferences               # List all preferences (super_admin only)
│   ├── GET /preferences/{userId}      # Get user preferences
│   ├── PUT /preferences               # Update user preferences
│   └── DELETE /preferences            # Delete user preferences
└── /config/
    ├── POST /config                   # Create system config
    ├── GET /config                    # List all configs (super_admin only)
    ├── GET /config/{context}          # Get specific config
    ├── PUT /config                    # Update system config
    └── DELETE /config                 # Delete system config
```

### Lambda Functions

#### 1. **UserHandler**
- **Purpose**: Manage user operations
- **Operations**: Read-only user access (users managed via Cognito)
- **Permissions**: Super admin can list all users, users can view own details

#### 2. **TemplateHandler**
- **Purpose**: Manage notification templates
- **Operations**: 
  - Create/update/delete templates
  - Support for global (*) and user-specific templates
  - Template inheritance (user templates override global)
- **Permissions**: Users manage own templates, super admin manages global templates

#### 3. **NotificationHandler** (Processor)
- **Purpose**: Process and send notifications
- **Operations**: 
  - Process immediate notifications via SQS
  - Apply template resolution and variable substitution
  - Handle multi-channel delivery
  - Record delivery validation for testing
- **Integrations**: SES, SNS, Slack webhooks, DynamoDB validation table

#### 4. **ScheduleHandler**
- **Purpose**: Manage scheduled notifications
- **Operations**: 
  - Create/update/delete/pause/resume schedules
  - Integrate with EventBridge Scheduler
  - Support cron expressions
- **Integrations**: EventBridge Scheduler for triggering

#### 5. **PreferenceHandler**
- **Purpose**: Manage user notification preferences
- **Operations**: 
  - Get/set user channel preferences
  - Support global defaults and user-specific overrides
  - Preference inheritance and merging

#### 6. **ConfigHandler**
- **Purpose**: Manage system configuration
- **Operations**: 
  - Manage channel-specific settings (Slack webhooks, email config, etc.)
  - Support global and user-specific configurations
  - Permission-based field access
- **Permissions**: Super admin for global config, users for own settings

### Data Models

#### User Model
```json
{
  "userId": "string (PK)",
  "email": "string",
  "role": "string", // "super_admin" | "user"
  "isActive": "boolean",
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

#### Template Model
```json
{
  "context": "string (PK)", // "*" | "<userid>"
  "type#channel": "string (SK)", // "alert#email" | "report#slack" | "notification#in_app"
  "content": "string", // Template with {{placeholders}}
  "isActive": "boolean",
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

#### User Preferences Model
```json
{
  "context": "string (PK)", // "*" | "<userid>"
  "preferences": {
    "alert": {
      "channels": ["email", "slack", "in_app"],
      "enabled": true
    },
    "report": {
      "channels": ["email"],
      "enabled": true
    },
    "notification": {
      "channels": ["in_app"],
      "enabled": true
    }
  },
  "timezone": "string",
  "language": "string",
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

#### Scheduled Notification Model
```json
{
  "scheduleId": "string (PK)",
  "userId": "string (GSI)",
  "type": "string", // "alert" | "report" | "notification"
  "variables": "object", // variables for the template
  "schedule": {
    "type": "string", // "cron"
    "expression": "string" // EventBridge Scheduler cron expression
  },
  "status": "string", // "active" | "paused" | "cancelled"
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

#### System Configuration Model
```json
{
  "context": "string (PK)", // "*" | "<userid>"
  "config": {
    "slack": {
      "webhookUrl": "string", // User-specific only
      "enabled": "boolean"
    },
    "email": {
      "fromAddress": "string", // Global only
      "replyToAddress": "string", // Global only
      "enabled": "boolean"
    },
    "inApp": {
      "platformAppIds": ["string"], // User-specific only
      "enabled": "boolean"
    }
  },
  "description": "string",
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

#### Notification Validation Model
```json
{
  "id#userId#type#channel": "string (PK)", // Composite key
  "content": "string", // Processed notification content
  "createdAt": "timestamp",
  "error": "string", // Error message if delivery failed
  "expiresAt": "number" // TTL - 1 day from creation
}
```

## Message Flow

### 1. Immediate Notification Flow
```
Client Request → API Gateway → NotificationHandler → SQS → ProcessorFunction → Channel Delivery → Validation Record
```

### 2. Scheduled Notification Flow
```
Client Request → API Gateway → ScheduleHandler → EventBridge Scheduler → (At Scheduled Time) → ProcessorFunction → Channel Delivery → Validation Record
```

### 3. Template Processing Flow
```
ProcessorFunction → DynamoDB (Get User Template) → [Fallback to Global Template] → Variable Substitution → Channel-Specific Formatting → Delivery
```

### 4. Preference Resolution Flow
```
ProcessorFunction → DynamoDB (Get User Preferences) → [Merge with Global Preferences] → Filter Enabled Channels → Deliver to Active Channels
```

### 5. Configuration Resolution Flow
```
ProcessorFunction → DynamoDB (Get User Config) → [Merge with Global Config] → Apply Channel Settings → Use for Delivery
```

## Security Architecture

### Authentication Flow
1. User authenticates via Cognito
2. Receives JWT token
3. All API calls include JWT in Authorization header
4. Lambda Authorizer validates JWT and extracts user info
5. Business logic enforces role-based permissions

### Authorization Levels
- **Super Admin**: 
  - Full system access
  - User management (read-only)
  - Global template management
  - Global configuration management
  - System monitoring and preferences access
- **User**: 
  - Own template management
  - Own preference management
  - Own configuration management (limited fields)
  - Sending notifications
  - Scheduled notification management

### Permission Matrix

| Resource | Super Admin | User |
|----------|-------------|------|
| Users (List) | ✅ | ❌ |
| Users (Own Details) | ✅ | ✅ |
| Global Templates | ✅ | ❌ (read-only via inheritance) |
| User Templates | ✅ | ✅ (own only) |
| Global Preferences | ✅ | ❌ |
| User Preferences | ✅ | ✅ (own only) |
| Global Config | ✅ | ❌ |
| User Config | ✅ | ✅ (own only, limited fields) |
| Send Notifications | ✅ | ✅ |
| Scheduled Notifications | ✅ | ✅ (own only) |

## Testing & Validation

### Notification Validation System
- **Purpose**: Track delivered notifications for testing and debugging
- **Storage**: DynamoDB with TTL (1 day expiration)
- **Key Structure**: `{notificationId}#{userId}#{type}#{channel}`
- **Content**: Processed notification content and any errors
- **Usage**: Automated tests verify notification delivery

### Test Framework
- **Python Test Suite**: Comprehensive API testing using pytest
- **User Management**: Test users created via Cognito
- **Test Coverage**: 
  - Authentication and authorization
  - Template CRUD operations
  - Preference management
  - System configuration
  - Notification sending and scheduling
  - Delivery verification via validation table

## Monitoring & Observability

### CloudWatch Metrics
- **API Gateway**: Request/response metrics, error rates
- **Lambda**: Duration, memory usage, error counts
- **DynamoDB**: Read/write capacity, throttling
- **SQS**: Message counts, processing times
- **EventBridge**: Rule executions, failures

### Logging
- **Structured Logging**: JSON format with correlation IDs
- **Log Levels**: ERROR, WARN, INFO, DEBUG
- **Sensitive Data**: Filtered from logs (emails, webhook URLs)

### Alarms
- **High Error Rates**: API Gateway 5xx errors > 5%
- **Lambda Failures**: Function error rate > 2%
- **DynamoDB Throttling**: Read/write throttling detected
- **SQS Dead Letter Queue**: Messages in DLQ > 0
- **Validation Table**: TTL deletion failures
