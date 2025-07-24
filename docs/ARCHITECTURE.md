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

#### 3. **Data Layer**
- **DynamoDB Tables**:
  - Users table
  - Templates table
  - User Preferences table
  - Scheduled Notifications table
  - System Configuration table

#### 4. **Messaging & Scheduling**
- **Amazon EventBridge**: Scheduled notification triggering
- **Amazon SQS**: Message queuing for notification processing
- **Amazon SNS**: In-app push notifications

#### 5. **Delivery Channels**
- **Amazon SES**: Email delivery
- **Slack Webhooks**: Slack message delivery
- **Amazon SNS**: Push notifications for mobile/web apps

#### 6. **Client Interface**
- **Python CLI Tool**: Command-line interface for API interactions

## Detailed Component Design

### API Gateway Structure

```
/api/v1/
├── /auth/
│   ├── POST /login
│   └── POST /refresh
├── /users/
│   ├── GET /users
│   ├── POST /users
│   ├── GET /users/{id}
│   ├── PUT /users/{id}
│   └── DELETE /users/{id}
├── /templates/
│   ├── GET /templates
│   ├── POST /templates
│   ├── GET /templates/{id}
│   ├── PUT /templates/{id}
│   └── DELETE /templates/{id}
├── /notifications/
│   ├── POST /send
│   ├── POST /schedule
│   ├── GET /scheduled
│   ├── PUT /scheduled/{id}
│   └── DELETE /scheduled/{id}
├── /preferences/
│   ├── GET /preferences/{userId}
│   └── PUT /preferences/{userId}
└── /config/
    ├── GET /config
    └── PUT /config
```

### Lambda Functions

#### 1. **UserHandler**
- **Purpose**: Manage user operations
- **Operations**: CRUD operations on users
- **Permissions**: Super admin only for user management

#### 2. **TemplateHandler**
- **Purpose**: Manage notification templates
- **Operations**: CRUD operations on templates
- **Permissions**: Users can manage their own templates

#### 3. **NotificationHandler**
- **Purpose**: Process and send notifications
- **Operations**: 
  - Send immediate notifications
  - Queue notifications for processing
- **Integrations**: SES, SNS, Slack webhooks

#### 4. **ScheduleHandler**
- **Purpose**: Manage scheduled notifications
- **Operations**: 
  - Create scheduled notifications
  - Cancel/modify schedules
- **Integrations**: EventBridge rules

#### 5. **PreferenceHandler**
- **Purpose**: Manage user notification preferences
- **Operations**: Get/set user channel preferences

#### 6. **ConfigHandler**
- **Purpose**: Manage system configuration
- **Operations**: Get/set system-wide settings (Slack webhooks, etc.)
- **Permissions**: Super admin only

#### 7. **ProcessorFunction**
- **Purpose**: Background notification processing
- **Trigger**: SQS messages
- **Operations**: 
  - Process queued notifications
  - Apply templates and variables
  - Deliver to appropriate channels

### Data Models

#### User Model
```json
{
  "userId": "string (PK)",
  "email": "string (GSI)",
  "role": "string", // "super_admin" | "user"
  "createdAt": "timestamp",
  "updatedAt": "timestamp",
  "isActive": "boolean"
}
```

#### Template Model
```json
{
  "context": "string (PK)", // "*" | "<userid>"
  "type#channel": "string (SK)", // "alert#email" | "report#email" | "notification#in_app"
  "name": "string",
  "content": "string", // Template with {{placeholders}}
  "variables": ["string"], // List of required variables
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

#### User Preferences Model
```json
{
  "context": "string (PK)", // "*" |"<userid>" , * is for global preferences
  "preferences": {
    "alerts": {
      "channels": ["email", "slack"],
      "enabled": true
    },
    "reports": {
      "channels": ["email"],
      "enabled": true
    },
    "notifications": {
      "channels": ["in_app"],
      "enabled": true
    }
  },
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
    "type": "string", // "one_time" | "recurring" | "cron"
    "expression": "string", // ISO timestamp or cron expression
    "timezone": "string"
  },
  "status": "string", // "active" | "paused" | "cancelled"
  "nextRun": "timestamp",
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

#### Configuration Model
```json
{
  "context": "string (PK)", // "*" | "<userid>" , * is for global preferences
  "type": "string", // "slack" | "in_app" | "email"
  "config": "object", // slack webhook url, in_app push token, email credentials
  "createdAt": "timestamp",
  "updatedAt": "timestamp"
}
```

## Message Flow

### 1. Immediate Notification Flow
```
Client Request → API Gateway → NotificationHandler → SQS → ProcessorFunction → Channel Delivery
```

### 2. Scheduled Notification Flow
```
Client Request → API Gateway → ScheduleHandler → EventBridge Rule → (At Scheduled Time) → ProcessorFunction → Channel Delivery
```

### 3. Template Processing Flow
```
ProcessorFunction → DynamoDB (Get Template) → Variable Substitution → Channel-Specific Formatting → Delivery
```

## Security Architecture

### Authentication Flow
1. User authenticates via Cognito
2. Receives JWT token
3. All API calls include JWT in Authorization header
4. Lambda Authorizer validates JWT and extracts user info
5. Business logic enforces role-based permissions

### Authorization Levels
- **Super Admin**: Full system access, user management, system configuration
- **User**: Own template management, sending notifications, preference management
