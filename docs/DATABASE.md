# Database Schema

## Overview

The notification service uses Amazon DynamoDB as the primary database, designed for high performance and scalability. The schema follows DynamoDB best practices with single-table design principles where appropriate.

## Tables

### 1. Users Table

**Table Name:** `notification-service-users`

**Primary Key:**
- Partition Key: `userId` (String)

**Global Secondary Indexes:**
- **GSI1**: `email` (Partition Key)
  - Purpose: User lookup by email address
  - Projection: ALL

**Attributes:**
```json
{
  "userId": "string",           // Unique user identifier
  "email": "string",           // User email (unique)
  "role": "string",            // "super_admin" | "user"
  "cognitoUserId": "string",   // Cognito user pool user ID
  "isActive": "boolean",       // Account status
  "createdAt": "string",       // ISO 8601 timestamp
  "updatedAt": "string",       // ISO 8601 timestamp
}
```

**Sample Record:**
```json
{
  "userId": "user-550e8400-e29b-41d4-a716-446655440000",
  "email": "john.doe@company.com",
  "role": "user",
  "cognitoUserId": "us-east-1:12345678-1234-1234-1234-123456789012",
  "isActive": true,
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

**Access Patterns:**
- Get user by ID: Query by `userId`
- Get user by email: Query GSI1 by `email`
- List all users: Scan (admin only, with pagination)

### 2. Templates Table

**Table Name:** `notification-service-templates`

**Primary Key:**
- Partition Key: `context` (String)
- Sort Key: `type#channel` (String)

**Attributes:**
```json
{
  "context": "string",      // "*" | "<userid>"
  "type#channel": "string", // "alert#email" | "report#email" | "notification#in_app"
  "name": "string",           // Template display name
  "content": "string",        // Template content with placeholders
  "variables": ["string"],    // Array of required variables
  "isActive": "boolean",      // Template status
  "createdAt": "string",      // ISO 8601 timestamp
  "updatedAt": "string"       // ISO 8601 timestamp
}
```

**Sample Record:**
```json
{
  "context": "user-550e8400-e29b-41d4-a716-446655440000",
  "type#channel": "alert#email",
  "name": "Server Down Alert",
  "content": "ALERT: Server {{serverName}} in {{environment}} is currently {{status}}. Immediate attention required.",
  "variables": ["serverName", "environment", "status"],
  "isActive": true,
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

**Access Patterns:**
- Get template by ID: Query by `context` and `type#channel`
- Get user's/global templates: Query by `context`
- Get templates by type: Query by `type`

### 3. User Preferences Table

**Table Name:** `notification-service-preferences`

**Primary Key:**
- Partition Key: `context` (String)

**Attributes:**
```json
{
  "context": "string",          // "*" | "<userid>"
  "preferences": {
    "alerts": {
      "channels": ["string"],  // Array of enabled channels
      "enabled": "boolean"     // Overall type enabled/disabled
    },
    "reports": {
      "channels": ["string"],
      "enabled": "boolean"
    },
    "notifications": {
      "channels": ["string"],
      "enabled": "boolean"
    }
  },
  "timezone": "string",        // User's preferred timezone
  "language": "string",        // Preferred language code
  "createdAt": "string",       // ISO 8601 timestamp
  "updatedAt": "string"        // ISO 8601 timestamp
}
```

**Sample Record:**
```json
{
  "context": "user-550e8400-e29b-41d4-a716-446655440000",
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
  "timezone": "UTC",
  "language": "en",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T11:45:00Z"
}
```

**Access Patterns:**
- Get user preferences: Query by `context = "<userid>"`
- Update user preferences: Update by `context = "<userid>"`
- Get global preferences: Query by `context = "*"`

### 4. Scheduled Notifications Table

**Table Name:** `notification-service-schedules`

**Primary Key:**
- Partition Key: `scheduleId` (String)

**Global Secondary Indexes:**
- **GSI1**: `userId` (Partition Key), `createdAt` (Sort Key)
  - Purpose: Get user's scheduled notifications
  - Projection: ALL
- **GSI2**: `status` (Partition Key), `nextRun` (Sort Key)
  - Purpose: Get active schedules for processing
  - Projection: ALL

**Attributes:**
```json
{
  "scheduleId": "string",      // Unique schedule identifier
  "userId": "string",          // Owner user ID
  "type": "string",           // "alert" | "report" | "notification"
  "variables": {},            // Template variables object
  "schedule": {
    "type": "string",         // "one_time" | "recurring" | "cron"
    "expression": "string",   // ISO timestamp or cron expression
    "timezone": "string"      // Timezone for scheduling
  },
  "status": "string",         // "active" | "paused" | "cancelled" | "completed"
  "nextRun": "string",        // ISO 8601 timestamp of next execution
  "lastRun": "string",        // ISO 8601 timestamp of last execution
  "runCount": "number",       // Number of times executed
  "eventBridgeRuleArn": "string", // EventBridge rule ARN
  "createdAt": "string",      // ISO 8601 timestamp
  "updatedAt": "string"       // ISO 8601 timestamp
}
```

**Sample Record:**
```json
{
  "scheduleId": "schedule-550e8400-e29b-41d4-a716-446655440002",
  "userId": "user-550e8400-e29b-41d4-a716-446655440000",
  "type": "report",
  "variables": {
    "reportType": "daily",
    "department": "engineering"
  },
  "schedule": {
    "type": "cron",
    "expression": "0 9 * * MON-FRI",
    "timezone": "UTC"
  },
  "status": "active",
  "nextRun": "2024-01-16T09:00:00Z",
  "lastRun": "2024-01-15T09:00:00Z",
  "runCount": 5,
  "eventBridgeRuleArn": "arn:aws:events:ap-south-1:123456789012:rule/notification-schedule-550e8400",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

**Access Patterns:**
- Get schedule by ID: Query by `scheduleId`
- Get user's schedules: Query GSI1 by `userId`
- Get active schedules for processing: Query GSI2 by `status = "active"`
- Get schedules by next run time: Query LSI1

### 5. System Configuration Table

**Table Name:** `notification-service-config`

**Primary Key:**
- Partition Key: `context` (String)

**Attributes:**
```json
{
  "context": "string",       // "*" | "<userid>"
  "config": {},          // Configuration value (can be any JSON type)
  "type": "string",      // "slack" | "in_app" | "email"
  "createdAt": "timestamp", // ISO 8601 timestamp
  "updatedAt": "timestamp"  // ISO 8601 timestamp
}
```

**Sample Records:**
```json
[
  {
    "context": "*",
    "config": {
      "slack": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"
    },
    "type": "slack",
    "createdAt": "2024-01-15T10:30:00Z",
    "updatedAt": "2024-01-15T10:30:00Z"
  },
  {
    "context": "*",
    "config": {
      "email": {
        "fromAddress": "notifications@company.com",
        "replyTo": "noreply@company.com",
        "defaultSubjectPrefix": "[Notification Service]"
      }
    },
    "type": "email",
    "createdAt": "2024-01-15T10:30:00Z",
    "updatedAt": "2024-01-15T10:30:00Z"
  }
]
```

**Access Patterns:**
- Get configuration by key: Query by `configKey`
- Get all configurations: Scan (admin only)

## DynamoDB Configuration

### Table Settings

**Billing Mode:** On-Demand
- Automatically scales based on traffic
- No need to provision read/write capacity
- Pay only for actual usage

**Encryption:** 
- Encryption at rest using AWS managed keys
- Encryption in transit via HTTPS/TLS

**Point-in-Time Recovery:** Enabled
- Continuous backups for 35 days
- Restore to any point in time

**Global Tables:** Not required initially
- Can be enabled for multi-region deployments

### Performance Considerations

**Hot Partitions:**
- User ID distribution ensures even partition usage
- Template access patterns spread across user partitions
- Schedule processing distributed by status and time

**Query Patterns:**
- Most queries use partition key for efficient access
- GSI queries provide required access patterns
- Scan operations limited to admin functions with pagination

**Item Size:**
- Users: ~1KB average
- Templates: ~2-5KB average
- Preferences: ~1KB average
- Schedules: ~2KB average
- Config: Variable (1-10KB)

### Backup Strategy

**Automated Backups:**
- Point-in-time recovery enabled
- Daily backups to S3 (if required for compliance)

**Cross-Region Backup:**
- Critical configuration replicated to secondary region
- User data backup for disaster recovery

### Security

**Access Control:**
- IAM roles with least privilege
- Lambda functions have table-specific permissions
- No direct database access from applications

**Data Classification:**
- PII: User emails (encrypted at rest)
- Sensitive: Configuration secrets (marked with isSecret flag)
- Public: Templates, preferences (non-sensitive)

### Monitoring

**CloudWatch Metrics:**
- Read/write capacity consumption
- Throttled requests
- System errors
- User errors

**Alarms:**
- High error rates
- Throttling events
- Unusual access patterns

### Migration Strategy

**Schema Evolution:**
- Backward-compatible changes preferred
- New attributes added without breaking existing records
- Migration scripts for structural changes

**Data Migration:**
- Blue-green deployment for major changes
- DynamoDB Streams for real-time replication
- AWS Data Migration Service for bulk transfers 