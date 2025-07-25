# Database Schema

## Overview

The notification service uses Amazon DynamoDB as the primary database, designed for high performance and scalability. The schema follows DynamoDB best practices with single-table design principles where appropriate.

## Tables

### 1. Users Table

**Table Name:** `notification-service-users`

**Primary Key:**
- Partition Key: `userId` (String)

**Attributes:**
```json
{
  "userId": "string",           // Unique user identifier (PK)
  "email": "string",           // User email
  "role": "string",            // "super_admin" | "user"
  "isActive": "boolean",       // Account status
  "createdAt": "string",       // ISO 8601 timestamp
  "updatedAt": "string"        // ISO 8601 timestamp
}
```

**Sample Record:**
```json
{
  "userId": "user-550e8400-e29b-41d4-a716-446655440000",
  "email": "john.doe@company.com",
  "role": "user",
  "isActive": true,
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

**Access Patterns:**
- Get user by ID: Query by `userId`
- List all users: Scan (admin only, with pagination)

### 2. Templates Table

**Table Name:** `notification-service-templates`

**Primary Key:**
- Partition Key: `context` (String)
- Sort Key: `type#channel` (String)

**Attributes:**
```json
{
  "context": "string",        // "*" for global templates | "<userid>" for user-specific
  "type#channel": "string",   // "alert#email" | "report#slack" | "notification#in_app"
  "content": "string",        // Template content with {{placeholders}}
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
  "content": "{\"subject\": \"Alert: {{serverName}} in {{environment}}\", \"body\": \"Server {{serverName}} in {{environment}} is {{status}} with message {{message}}\"}",
  "isActive": true,
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

**Access Patterns:**
- Get template by context and type#channel: Query by `context` and `type#channel`
- Get templates by context: Query by `context`
- List templates for user/global: Query by `context`

### 3. User Preferences Table

**Table Name:** `notification-service-preferences`

**Primary Key:**
- Partition Key: `context` (String)

**Attributes:**
```json
{
  "context": "string",          // "*" for global | "<userid>" for user-specific
  "preferences": {
    "alert": {
      "channels": ["string"],  // Array of enabled channels
      "enabled": "boolean"     // Overall type enabled/disabled
    },
    "report": {
      "channels": ["string"],
      "enabled": "boolean"
    },
    "notification": {
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
    "alert": {
      "channels": ["email", "slack"],
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
- **UserIndex**: `userId` (Partition Key), `createdAt` (Sort Key)
  - Purpose: Get user's scheduled notifications
  - Projection: ALL

**Attributes:**
```json
{
  "scheduleId": "string",      // Unique schedule identifier (PK)
  "userId": "string",          // Owner user ID (GSI)
  "type": "string",           // "alert" | "report" | "notification"
  "variables": {},            // Template variables object
  "schedule": {
    "type": "string",         // "cron"
    "expression": "string"    // Cron expression (EventBridge Scheduler format)
  },
  "status": "string",         // "active" | "paused" | "cancelled" | "completed"
  "createdAt": "string",      // ISO 8601 timestamp
  "updatedAt": "string"       // ISO 8601 timestamp
}
```

**Sample Record:**
```json
{
  "scheduleId": "schedule-550e8400-e29b-41d4-a716-446655440002",
  "userId": "user-550e8400-e29b-41d4-a716-446655440000",
  "type": "alert",
  "variables": {
    "serverName": "web-server-01",
    "environment": "production",
    "status": "critical",
    "message": "High CPU usage"
  },
  "schedule": {
    "type": "cron",
    "expression": "0 9 * * ? *"
  },
  "status": "active",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

**Access Patterns:**
- Get schedule by ID: Query by `scheduleId`
- Get user's schedules: Query UserIndex by `userId`
- List all schedules: Scan (admin only, with pagination)
- Get active schedules: Scan with filter by `status = "active"`

### 5. System Configuration Table

**Table Name:** `notification-service-config`

**Primary Key:**
- Partition Key: `context` (String)

**Attributes:**
```json
{
  "context": "string",       // "*" for global | "<userid>" for user-specific
  "config": {
    "slack": {
      "webhookUrl": "string",
      "enabled": "boolean"
    },
    "email": {
      "fromAddress": "string",
      "replyToAddress": "string",
      "enabled": "boolean"
    },
    "inApp": {
      "platformAppIds": ["string"],
      "enabled": "boolean"
    }
  },
  "description": "string",      // Configuration description
  "createdAt": "string",        // ISO 8601 timestamp
  "updatedAt": "string"         // ISO 8601 timestamp
}
```

**Sample Records:**
```json
[
  {
    "context": "*",
    "config": {
      "slack": {
        "enabled": true
      },
      "email": {
        "fromAddress": "notifications@company.com",
        "replyToAddress": "noreply@company.com",
        "enabled": true
      },
      "inApp": {
        "enabled": true
      }
    },
    "description": "Global system configuration",
    "createdAt": "2024-01-15T10:30:00Z",
    "updatedAt": "2024-01-15T10:30:00Z"
  },
  {
    "context": "user-550e8400-e29b-41d4-a716-446655440000",
    "config": {
      "slack": {
        "webhookUrl": "https://hooks.slack.com/services/USER/WEBHOOK/URL",
        "enabled": true
      },
      "inApp": {
        "platformAppIds": ["app1", "app2"],
        "enabled": true
      }
    },
    "description": "User-specific configuration",
    "createdAt": "2024-01-15T11:00:00Z",
    "updatedAt": "2024-01-15T11:00:00Z"
  }
]
```

**Access Patterns:**
- Get configuration by context: Query by `context`
- List all configurations: Scan (admin only)

### 6. Notification Validation Table

**Table Name:** `notification-service-validation`

**Primary Key:**
- Partition Key: `id#userId#type#channel` (String)

**TTL Attribute:** `expiresAt` (Number) - Records expire after 1 day

**Attributes:**
```json
{
  "id#userId#type#channel": "string", // Composite key: notificationId#userId#type#channel
  "content": "string",                 // Processed notification content
  "createdAt": "string",              // ISO 8601 timestamp
  "error": "string",                  // Error message if delivery failed
  "expiresAt": "number"               // Unix timestamp for TTL (1 day from creation)
}
```

**Sample Record:**
```json
{
  "id#userId#type#channel": "alert-123#user-456#alert#email",
  "content": "Alert: web-server-01 is critical in production with message High CPU usage",
  "createdAt": "2024-01-15T10:30:00Z",
  "expiresAt": 1705406600
}
```

**Access Patterns:**
- Get validation by composite key: Query by `id#userId#type#channel`
- Records automatically expire after 1 day (TTL)
- Used for testing and delivery verification

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
- Schedule processing distributed by user and time
- Validation table uses composite keys for distribution

**Query Patterns:**
- Most queries use partition key for efficient access
- GSI queries provide required access patterns for user-specific data
- Scan operations limited to admin functions with pagination
- TTL automatically manages validation data lifecycle

**Item Size:**
- Users: ~1KB average
- Templates: ~2-5KB average
- Preferences: ~1-2KB average
- Schedules: ~1-3KB average
- Config: 1-10KB depending on configuration complexity
- Validation: ~1KB average

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
- Sensitive: Configuration secrets (Slack webhooks, etc.)
- Public: Templates, preferences (non-sensitive)

### Monitoring

**CloudWatch Metrics:**
- Read/write capacity consumption
- Throttled requests
- System errors
- User errors
- TTL deletes (validation table)

**Alarms:**
- High error rates
- Throttling events
- Unusual access patterns
- TTL processing issues

### Migration Strategy

**Schema Evolution:**
- Backward-compatible changes preferred
- New attributes added without breaking existing records
- Migration scripts for structural changes

**Data Migration:**
- Blue-green deployment for major changes
- DynamoDB Streams for real-time replication
- AWS Data Migration Service for bulk transfers 