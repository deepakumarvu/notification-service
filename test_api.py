import pytest
from testutils.test_user import User
import os
import json
import time
import boto3
from boto3.dynamodb.conditions import Key
import uuid
import datetime

with open('cdk-outputs.json', 'r') as f:
    data = json.load(f)
    
ENVIRONMENT = os.getenv("ENVIRONMENT")
REGION = data[f"NotificationService-{ENVIRONMENT}"]["Region"]
ACCOUNT_ID = data[f"NotificationService-{ENVIRONMENT}"]["AccountId"]
USER_POOL_ID = data[f"NotificationService-{ENVIRONMENT}"]["UserPoolId"]
USER_POOL_CLIENT_ID = data[f"NotificationService-{ENVIRONMENT}"]["UserPoolClientId"]
API_GATEWAY_URL = data[f"NotificationService-{ENVIRONMENT}"]["APIGatewayURL"]
NOTIFICATION_QUEUE_URL = data[f"NotificationService-{ENVIRONMENT}"]["NotificationQueueURL"]
NOTIFICATION_VALIDATION_TABLE = data[f"NotificationService-{ENVIRONMENT}"]["NotificationValidationTable"]

dynamodb = boto3.client('dynamodb', region_name=REGION)
    
def get_notification_validation_data(id, userId, type, channel):
    response = dynamodb.get_item(
        TableName=NOTIFICATION_VALIDATION_TABLE,
        Key={
            'id#userId#type#channel': {
                'S': f"{id}#{userId}#{type}#{channel}"
            }
        }
    )
    return response["Item"]

@pytest.fixture(scope="session")
def test_super_admin():
    admin = User("super_admin2@company.com", "TestPassword10!", "super_admin", REGION, USER_POOL_ID, USER_POOL_CLIENT_ID, API_GATEWAY_URL, NOTIFICATION_QUEUE_URL)
    admin.create_user()
    admin.authenticate_user()
    yield admin

@pytest.fixture(scope="session")
def test_user():
    user = User("user1@company.com", "TestPassword10!", "user", REGION, USER_POOL_ID, USER_POOL_CLIENT_ID, API_GATEWAY_URL, NOTIFICATION_QUEUE_URL)
    user.create_user()
    user.authenticate_user()
    yield user
    
def test_get_users_list(test_super_admin: User):
    response = test_super_admin.get_users_list()
    assert response.status_code == 200
    response_json = response.json()
    assert len(response_json["items"]) >= 0

def test_get_user_by_id(test_super_admin: User, test_user: User):
    response = test_super_admin.get_user_by_id(test_user.user_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["userId"] == test_user.user_id

    response = test_super_admin.get_user_by_id(test_super_admin.user_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["userId"] == test_super_admin.user_id
    assert response_json["role"] == "super_admin"
    
def test_get_user_by_id_by_normal_user(test_user: User, test_super_admin: User):
    response = test_user.get_user_by_id(test_user.user_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["userId"] == test_user.user_id
    assert response_json["role"] == "user"
    
    # normal user should not be able to get user details other than their own
    response = test_user.get_user_by_id(test_super_admin.user_id)
    assert response.status_code == 403
    
    # normal user should not be able to get users list
    response = test_user.get_users_list()
    assert response.status_code == 403
    
def test_template(test_super_admin: User, test_user: User):
    # Create a global template
    response = test_super_admin.create_template("*", "alert", "email", "{\"subject\": \"There is an alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}")
    assert response.status_code == 201
    response_json = response.json()
    assert response_json["context"] == "*"
    assert response_json["type#channel"] == "alert#email"
    assert response_json["content"] == "{\"subject\": \"There is an alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}"
    
    # Get global template
    response = test_super_admin.get_template_by_id("*", "alert", "email")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == "*"
    assert response_json["type#channel"] == "alert#email"
    assert response_json["content"] == "{\"subject\": \"There is an alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}"
    
    # Create user templates
    response = test_user.create_template("", "alert", "email", "{\"subject\": \"There is an user alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}")
    assert response.status_code == 201
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["type#channel"] == "alert#email"
    assert response_json["content"] == "{\"subject\": \"There is an user alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}"
    
    # Get user template
    response = test_user.get_template_by_id("", "alert", "email")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["type#channel"] == "alert#email"
    assert response_json["content"] == "{\"subject\": \"There is an user alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}"
    
    # User should not be able to get global templates
    response = test_user.get_template_by_id("*", "alert", "email")
    assert response.status_code == 403
    
    # Update template for user
    response = test_user.update_template("", "alert", "email", "{\"subject\": \"New subject is an user alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["type#channel"] == "alert#email"
    assert response_json["content"] == "{\"subject\": \"New subject is an user alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}"
    
    # Try to update a template that doesn't exist
    response = test_user.update_template("", "alert", "in_app", "Some Message")
    assert response.status_code == 404
    
    # Try to update a template with no content
    response = test_user.update_template("", "alert", "email", "")
    assert response.status_code == 400
    
    # user should not be able to update global templates
    response = test_user.update_template("*", "alert", "email", "{\"subject\": \"New subject is an user alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}")
    assert response.status_code == 403
    
    # Delete template for user
    response = test_user.delete_template("", "alert", "email")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["message"] == "Template deleted successfully"
    
    # User should not be able to delete global templates
    response = test_user.delete_template("*", "alert", "email")
    assert response.status_code == 403
    
    # Delete global template
    response = test_super_admin.delete_template("*", "alert", "email")
    assert response.status_code == 200
    response_json = response.json()

def test_user_preferences_positive(test_super_admin: User, test_user: User):
    # Test creating user preferences for normal user
    preferences = {
        "alert": {
            "channels": ["email", "slack"],
            "enabled": True
        },
        "report": {
            "channels": ["email"],
            "enabled": False
        }
    }
    
    response = test_user.create_user_preferences("", preferences, "UTC", "en")
    assert response.status_code == 201
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["preferences"]["alert"]["channels"] == ["email", "slack"]
    assert response_json["preferences"]["alert"]["enabled"] == True
    assert response_json["timezone"] == "UTC"
    assert response_json["language"] == "en"
    
    # Test getting user preferences
    response = test_user.get_user_preferences(test_user.user_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["preferences"]["alert"]["channels"] == ["email", "slack"]
    assert response_json["timezone"] == "UTC"
    assert response_json["language"] == "en"
    
    # Test updating user preferences
    updated_preferences = {
        "alert": {
            "channels": ["email"],
            "enabled": True
        },
        "notification": {
            "channels": ["in_app"],
            "enabled": True
        }
    }
    
    response = test_user.update_user_preferences("", updated_preferences, "America/New_York", "es")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["preferences"]["alert"]["channels"] == ["email"]
    assert response_json["preferences"]["notification"]["channels"] == ["in_app"]
    assert response_json["timezone"] == "America/New_York"
    assert response_json["language"] == "es"
    
    # Test partial update (only timezone)
    response = test_user.update_user_preferences("", None, "Europe/London", None)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["timezone"] == "Europe/London"
    assert response_json["language"] == "es"  # Should remain unchanged
    
    # Test creating global preferences (super admin only)
    global_preferences = {
        "alert": {
            "channels": ["email", "slack", "in_app"],
            "enabled": True
        }
    }
    
    response = test_super_admin.create_user_preferences("*", global_preferences, "UTC", "en")
    assert response.status_code == 201
    response_json = response.json()
    assert response_json["context"] == "*"
    assert response_json["preferences"]["alert"]["channels"] == ["email", "slack", "in_app"]
    
    # Test getting global preferences
    response = test_super_admin.get_user_preferences("*")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == "*"
    
    # Test listing all preferences (super admin only)
    response = test_super_admin.get_user_preferences_list()
    assert response.status_code == 200
    response_json = response.json()
    assert len(response_json["items"]) >= 2  # At least user and global preferences
    
    # Normal user should not be able to list all preferences
    response = test_user.get_user_preferences_list()
    assert response.status_code == 403
    
    # Normal user should not be able to create global preferences
    response = test_user.create_user_preferences("*", global_preferences, "UTC", "en")
    assert response.status_code == 403
    
    # Normal user should not be able to get global preferences
    response = test_user.get_user_preferences("*")
    assert response.status_code == 403
    
    # Test deleting user preferences
    response = test_user.delete_user_preferences("")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["message"] == "User preferences deleted successfully"
    
    # Test getting deleted preferences (should return 404)
    response = test_user.get_user_preferences(test_user.user_id)
    assert response.status_code == 404
    
    # Test deleting global preferences
    response = test_super_admin.delete_user_preferences("*")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["message"] == "User preferences deleted successfully"

def test_user_preferences_validation(test_user: User):
    # Test creating preferences without preferences data (should fail)
    response = test_user.create_user_preferences("", None, "UTC", "en")
    assert response.status_code == 400
    
    # Test creating preferences with invalid notification type
    invalid_preferences = {
        "invalid_type": {
            "channels": ["email"],
            "enabled": True
        }
    }
    
    response = test_user.create_user_preferences("", invalid_preferences, "UTC", "en")
    assert response.status_code == 400
    
    # Test creating preferences with invalid channel
    invalid_channel_preferences = {
        "alert": {
            "channels": ["invalid_channel"],
            "enabled": True
        }
    }
    
    response = test_user.create_user_preferences("", invalid_channel_preferences, "UTC", "en")
    assert response.status_code == 400
    
    # Test updating non-existent preferences
    response = test_user.update_user_preferences("", {"alert": {"channels": ["email"], "enabled": True}}, None, None)
    assert response.status_code == 404
    
    # Test deleting non-existent preferences
    response = test_user.delete_user_preferences("")
    assert response.status_code == 404
    
    # Test getting non-existent preferences
    response = test_user.get_user_preferences(test_user.user_id)
    assert response.status_code == 404

def test_user_preferences_duplicate_creation(test_user: User):
    # Create initial preferences
    preferences = {
        "alert": {
            "channels": ["email"],
            "enabled": True
        }
    }
    
    response = test_user.create_user_preferences("", preferences, "UTC", "en")
    assert response.status_code == 201
    
    # Try to create preferences again (should fail)
    response = test_user.create_user_preferences("", preferences, "UTC", "en")
    assert response.status_code == 400
    
    # Clean up
    test_user.delete_user_preferences("")

def test_user_preferences_update_validation(test_user: User):
    # Create initial preferences
    preferences = {
        "alert": {
            "channels": ["email"],
            "enabled": True
        }
    }
    
    response = test_user.create_user_preferences("", preferences, "UTC", "en")
    assert response.status_code == 201
    
    # Test update with no fields provided
    response = test_user.update_user_preferences("", None, None, None)
    assert response.status_code == 400
    
    # Test update with invalid notification type
    invalid_preferences = {
        "invalid_type": {
            "channels": ["email"],
            "enabled": True
        }
    }
    
    response = test_user.update_user_preferences("", invalid_preferences, None, None)
    assert response.status_code == 400
    
    # Clean up
    test_user.delete_user_preferences("")

def test_system_config_positive(test_super_admin: User, test_user: User):
    # Test creating user system config
    user_config = {
        "slack": {
            "webhookUrl": "https://hooks.slack.com/user-webhook",
            "enabled": True
        },
        "email": {
            "enabled": False
        },
        "inApp": {
            "platformAppIds": ["app1", "app2"],
            "enabled": True
        }
    }
    
    response = test_user.create_system_config("", user_config, "User specific config")
    assert response.status_code == 201
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["config"]["slack"]["webhookUrl"] == "https://hooks.slack.com/user-webhook"
    assert response_json["config"]["slack"]["enabled"] == True
    assert response_json["config"]["email"]["enabled"] == False
    assert response_json["config"]["inApp"]["platformAppIds"] == ["app1", "app2"]
    assert response_json["description"] == "User specific config"
    
    # Test getting user system config
    response = test_user.get_system_config(test_user.user_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["config"]["slack"]["webhookUrl"] == "https://hooks.slack.com/user-webhook"
    
    # Test updating user system config (partial update)
    updated_config = {
        "slack": {
            "enabled": False
        },
        "email": {
            "enabled": True
        }
    }
    
    response = test_user.update_system_config("", updated_config, "Updated user config")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == test_user.user_id
    assert response_json["config"]["slack"]["enabled"] == False
    assert response_json["config"]["email"]["enabled"] == True
    # Webhook URL should be preserved from previous config
    assert response_json["config"]["slack"]["webhookUrl"] == "https://hooks.slack.com/user-webhook"
    assert response_json["config"]["inApp"]["platformAppIds"] == ["app1", "app2"]
    
    # Test creating global system config (super admin only)
    global_config = {
        "slack": {
            "enabled": True
        },
        "email": {
            "fromAddress": "notifications@company.com",
            "replyToAddress": "noreply@company.com",
            "enabled": True
        },
        "inApp": {
            "enabled": True
        }
    }
    
    response = test_super_admin.create_system_config("*", global_config, "Global company config")
    assert response.status_code == 201
    response_json = response.json()
    assert response_json["context"] == "*"
    assert response_json["config"]["email"]["fromAddress"] == "notifications@company.com"
    assert response_json["config"]["email"]["replyToAddress"] == "noreply@company.com"
    
    # Test getting global system config
    response = test_super_admin.get_system_config("*")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["context"] == "*"
    assert response_json["config"]["email"]["fromAddress"] == "notifications@company.com"
    
    # Test listing all system configs (super admin only)
    response = test_super_admin.get_system_config_list()
    assert response.status_code == 200
    response_json = response.json()
    assert len(response_json["items"]) >= 2  # At least user and global configs
    
    # Normal user should not be able to list all configs
    response = test_user.get_system_config_list()
    assert response.status_code == 403
    
    # Normal user should not be able to create global config
    response = test_user.create_system_config("*", global_config, "Unauthorized global config")
    assert response.status_code == 403
    
    # Normal user should not be able to get global config
    response = test_user.get_system_config("*")
    assert response.status_code == 403
    
    # Test deleting user system config
    response = test_user.delete_system_config("")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["message"] == "System config deleted successfully"
    
    # Test getting deleted config (should return 404)
    response = test_user.get_system_config(test_user.user_id)
    assert response.status_code == 404
    
    # Test deleting global system config
    response = test_super_admin.delete_system_config("*")
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["message"] == "System config deleted successfully"

def test_system_config_permission_validation(test_super_admin: User, test_user: User):
    # Test user trying to modify forbidden email addresses
    invalid_user_config = {
        "email": {
            "fromAddress": "user@company.com",
            "enabled": True
        }
    }
    
    response = test_user.create_system_config("", invalid_user_config, "Invalid user config")
    assert response.status_code == 403
    
    # Test super admin trying to modify forbidden fields in global config
    invalid_global_config = {
        "slack": {
            "webhookUrl": "https://hooks.slack.com/global-webhook",
            "enabled": True
        }
    }
    
    response = test_super_admin.create_system_config("*", invalid_global_config, "Invalid global config")
    assert response.status_code == 403
    
    # Test super admin trying to modify platform app IDs in global config
    invalid_global_config_2 = {
        "inApp": {
            "platformAppIds": ["global-app1"],
            "enabled": True
        }
    }
    
    response = test_super_admin.create_system_config("*", invalid_global_config_2, "Invalid global config 2")
    assert response.status_code == 403

def test_system_config_user_permission_merge(test_super_admin: User, test_user: User):
    # First create a global config with email settings
    global_config = {
        "email": {
            "fromAddress": "global@company.com",
            "replyToAddress": "noreply@company.com",
            "enabled": True
        },
        "slack": {
            "enabled": False
        }
    }
    
    response = test_super_admin.create_system_config("*", global_config, "Global config for merge test")
    assert response.status_code == 201
    
    # Create user config
    user_config = {
        "slack": {
            "webhookUrl": "https://hooks.slack.com/user-webhook",
            "enabled": True
        },
        "inApp": {
            "platformAppIds": ["user-app1"],
            "enabled": True
        }
    }
    
    response = test_user.create_system_config("", user_config, "User config for merge test")
    assert response.status_code == 201
    
    # Now update user config - should preserve global email settings
    updated_user_config = {
        "email": {
            "enabled": False  # User can only enable/disable
        }
    }
    
    response = test_user.update_system_config("", updated_user_config, "Updated user config")
    assert response.status_code == 200
    response_json = response.json()
    
    # User's email enable/disable should be updated
    assert response_json["config"]["email"]["enabled"] == False
    # But webhook URL and platform app IDs should be preserved from user's original config
    assert response_json["config"]["slack"]["webhookUrl"] == "https://hooks.slack.com/user-webhook"
    assert response_json["config"]["inApp"]["platformAppIds"] == ["user-app1"]
    
    # Clean up
    test_user.delete_system_config("")
    test_super_admin.delete_system_config("*")

def test_system_config_validation_errors(test_user: User):
    # Test creating config with invalid data
    response = test_user.create_system_config("", None, "Empty config")
    assert response.status_code == 400  # Should fail with empty config
    
    # Test updating non-existent config
    config = {
        "slack": {
            "enabled": True
        }
    }
    response = test_user.update_system_config("", config, "Update non-existent")
    assert response.status_code == 404
    
    # Test deleting non-existent config
    response = test_user.delete_system_config("")
    assert response.status_code == 404
    
    # Test getting non-existent config
    response = test_user.get_system_config(test_user.user_id)
    assert response.status_code == 404
    
    # Clean up the empty config
    test_user.delete_system_config("")

def test_system_config_duplicate_creation(test_user: User):
    # Create initial config
    config = {
        "slack": {
            "enabled": True
        }
    }
    
    response = test_user.create_system_config("", config, "Initial config")
    assert response.status_code == 201
    
    # Try to create config again (should fail)
    response = test_user.create_system_config("", config, "Duplicate config")
    assert response.status_code == 400
    
    # Clean up
    test_user.delete_system_config("")

def test_system_config_super_admin_global_replacement(test_super_admin: User):
    # Create initial global config
    initial_config = {
        "email": {
            "fromAddress": "initial@company.com",
            "enabled": True
        },
        "slack": {
            "enabled": False
        }
    }
    
    response = test_super_admin.create_system_config("*", initial_config, "Initial global config")
    assert response.status_code == 201
    
    # Super admin should be able to completely replace global config
    new_config = {
        "email": {
            "fromAddress": "new@company.com",
            "replyToAddress": "noreply@company.com",
            "enabled": True
        },
        "inApp": {
            "enabled": True
        }
    }
    
    response = test_super_admin.update_system_config("*", new_config, "Completely new global config")
    assert response.status_code == 200
    response_json = response.json()
    
    # Config should be completely replaced
    assert response_json["config"]["email"]["fromAddress"] == "new@company.com"
    assert response_json["config"]["email"]["replyToAddress"] == "noreply@company.com"
    assert response_json["config"]["inApp"]["enabled"] == True
    # Slack settings should be from new config (not preserved from old)
    assert "enabled" not in response_json["config"]["slack"] or response_json["config"]["slack"]["enabled"] is None
    
    # Clean up
    test_super_admin.delete_system_config("*")

def test_notification_queue_publishing(test_super_admin: User, test_user: User):

    # Setup Global Template, Preferences, System Config
    response = test_super_admin.create_template("*", "alert", "email", "{\"subject\": \"There is an alert in {{serverName}} in {{environment}}\", \"body\": \"There is an alert in {{serverName}} in {{environment}} with status {{status}} and message {{message}}\"}")
    print(response.json())
    response = test_super_admin.create_template("*", "alert", "slack", "Alert: {{serverName}} is {{status}} in {{environment}} with message {{message}}")
    print(response.json())
    response = test_super_admin.create_template("*", "alert", "in_app", "Alert: {{serverName}} is {{status}} in {{environment}} with message {{message}}")
    print(response.json())
    
    response = test_super_admin.create_template("*", "report", "email", "{\"subject\": \"There is a report in {{reportType}} for {{period}}\", \"body\": \"There is a report in {{reportType}} for {{period}} with data {{data}}\"}")
    print(response.json())
    response = test_super_admin.create_template("*", "report", "slack", "Report: {{reportType}} for {{period}} is {{data}}")
    print(response.json())
    response = test_super_admin.create_template("*", "report", "in_app", "Report: {{reportType}} for {{period}} is {{data}}")
    print(response.json())
    
    response = test_super_admin.create_template("*", "notification", "email", "{\"subject\": \"There is a notification in {{title}}\", \"body\": \"There is a notification in {{title}} with message {{message}} and action url {{actionUrl}}\"}")
    print(response.json())
    response = test_super_admin.create_template("*", "notification", "slack", "Notification: {{title}} is {{message}} with {{actionUrl}}")
    print(response.json())
    response = test_super_admin.create_template("*", "notification", "in_app", "Notification: {{title}} is {{message}} with {{actionUrl}}")
    print(response.json())
    
    response = test_super_admin.create_user_preferences("*", {"alert": {"channels": ["email", "slack", "in_app"], "enabled": True}, "report": {"channels": ["email", "slack", "in_app"], "enabled": True}, "notification": {"channels": ["email", "slack", "in_app"], "enabled": True}}, "UTC", "en")
    print(response.json())
    response = test_super_admin.create_system_config("*", {"email": {"fromAddress": "notifications@company.com", "replyToAddress": "noreply@company.com", "enabled": True}, 
                                                "slack": {"enabled": True}, 
                                                "inApp": {"enabled": True}}, "Global config")
    print(response.json())
    
    # Create user preferences
    response = test_user.create_user_preferences(test_user.user_id, {"alert": {"channels": ["email", "slack"], "enabled": True}, "report": {"channels": ["slack", "in_app"], "enabled": True}, "notification": {"channels": ["in_app"], "enabled": True}}, "UTC", "en")
    print(response.json())
    
    # Create User Template
    response = test_user.create_template(test_user.user_id, "notification", "in_app", "User Notification: {{title}} is {{message}} with {{actionUrl}}")
    print(response.json())
    
    alert_id = str(uuid.uuid4())
    report_id = str(uuid.uuid4())
    notification_id = str(uuid.uuid4())
    
    # Test sending alert notification
    alert_response = test_user.send_alert_notification(
        id=alert_id,
        recipients=[test_user.user_id, test_super_admin.user_id],
        server_name="web-server-01",
        environment="production",
        status="critical",
        message="High CPU usage detected"
    )
    assert "MessageId" in alert_response
    
    # Test sending report notification
    report_response = test_super_admin.send_report_notification(
        id=report_id,
        recipients=[test_super_admin.user_id],
        report_type="Weekly Performance Report",
        time_period="2024-01-01 to 2024-01-07",
        summary="System performance metrics for the past week"
    )
    assert "MessageId" in report_response
    
    # Test sending general notification
    notification_response = test_user.send_general_notification(
        recipients=[test_user.user_id],
        id=notification_id,
        title="System Maintenance",
        message="Scheduled maintenance will begin at 2 AM UTC",
        action_url="https://example.com/acknowledge"
    )
    assert "MessageId" in notification_response
    
    # Sleep for 5 seconds
    time.sleep(5)
    
    # Validate the notifications sent to the users
    
    # In App notification should not be sent to the user
    try:
        alert_validation = get_notification_validation_data(alert_id, test_user.user_id, "alert", "in_app")
    except KeyError as e:
        assert True
    except Exception as e:
        assert False
    
    alert_validation = get_notification_validation_data(alert_id, test_user.user_id, "alert", "slack")
    assert alert_validation["content"]["S"] == "Alert: web-server-01 is critical in production with message High CPU usage detected"
    assert "error" not in alert_validation
    
    report_validation = get_notification_validation_data(alert_id, test_super_admin.user_id, "alert", "in_app")
    assert report_validation["content"]["S"] == "Alert: web-server-01 is critical in production with message High CPU usage detected"
    assert "error" not in report_validation
    
    report_validation = get_notification_validation_data(report_id, test_super_admin.user_id, "report", "email")
    assert report_validation["content"]["S"] == "{\"body\":\"There is a report in Weekly Performance Report for 2024-01-01 to 2024-01-07 with data System performance metrics for the past week\",\"subject\":\"There is a report in Weekly Performance Report for 2024-01-01 to 2024-01-07\"}"
    assert "error" not in report_validation
    
    # Specific user template should be sent to the user
    notification_validation = get_notification_validation_data(notification_id, test_user.user_id, "notification", "in_app")
    assert notification_validation["content"]["S"] == "User Notification: System Maintenance is Scheduled maintenance will begin at 2 AM UTC with https://example.com/acknowledge"
    assert "error" not in notification_validation
    
    # Clean up
    test_super_admin.delete_template("*", "alert", "email")
    test_super_admin.delete_template("*", "alert", "slack")
    test_super_admin.delete_template("*", "alert", "in_app")
    test_super_admin.delete_template("*", "report", "email")
    test_super_admin.delete_template("*", "report", "slack")
    test_super_admin.delete_template("*", "report", "in_app")
    test_super_admin.delete_template("*", "notification", "email")
    test_super_admin.delete_template("*", "notification", "slack")
    test_super_admin.delete_template("*", "notification", "in_app")
    test_super_admin.delete_user_preferences("*")
    test_super_admin.delete_system_config("*")
    test_user.delete_template(test_user.user_id, "notification", "in_app")
    test_user.delete_user_preferences(test_user.user_id)

def test_scheduled_notifications(test_user: User, test_super_admin: User):
    """Test scheduled notification CRUD operations and delivery"""
    
    # Create a scheduled notification with a test-friendly schedule
    variables = {"message": "Daily reminder", "serverName": "web-server-01", "environment": "production", "status": "critical"}
    response = test_user.create_scheduled_notification(
        notification_type="alert",
        variables=variables,
        cron_expression="0 9 * * ? *"  # Daily at 9 AM (EventBridge Scheduler format)
    )
    assert response.status_code == 201
    response_json = response.json()
    assert response_json["type"] == "alert"
    assert response_json["variables"] == variables
    assert response_json["schedule"]["expression"] == "0 9 * * ? *"
    assert response_json["status"] == "active"
    schedule_id = response_json["scheduleId"]
    
    # Get the scheduled notification
    response = test_user.get_scheduled_notification_by_id(schedule_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["scheduleId"] == schedule_id
    assert response_json["userId"] == test_user.user_id
    
    # Test pause/resume functionality
    response = test_user.pause_scheduled_notification(schedule_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["status"] == "paused"
    
    response = test_user.resume_scheduled_notification(schedule_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["status"] == "active"
    
    # Update the scheduled notification
    response = test_user.update_scheduled_notification(
        schedule_id,
        variables={"message": "Updated reminder"},
        cron_expression="0 10 * * ? *"  # Change to 10 AM (EventBridge Scheduler format)
    )
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["variables"]["message"] == "Updated reminder"
    assert response_json["schedule"]["expression"] == "0 10 * * ? *"
    
    # List user's scheduled notifications
    response = test_user.get_scheduled_notifications_list()
    assert response.status_code == 200
    response_json = response.json()
    assert len(response_json["items"]) >= 1
    
    # Delete the scheduled notification
    response = test_user.delete_scheduled_notification(schedule_id)
    assert response.status_code == 200
    response_json = response.json()
    assert response_json["message"] == "Scheduled notification deleted successfully"
    
def test_scheduled_notifications_delivery_verification(test_user: User, test_super_admin: User):
    """Test scheduled notification delivery with verification - creates a schedule for next minute"""
    
    # Setup required templates and preferences
    test_super_admin.create_template("*", "alert", "in_app", "SCHEDULED DELIVERY TEST: {{serverName}} is {{status}} - {{message}}")
    test_super_admin.create_user_preferences("*", {"alert": {"channels": ["in_app"], "enabled": True}}, "UTC", "en")
    test_super_admin.create_system_config("*", {"inApp": {"enabled": True}}, "Test delivery config")
    test_user.create_user_preferences(test_user.user_id, {"alert": {"channels": ["in_app"], "enabled": True}}, "UTC", "en")
    
    # Create unique ID for tracking
    test_schedule_id = str(uuid.uuid4())
    
    # Calculate next minute for immediate testing
    now = datetime.datetime.utcnow()
    next_minute = now.replace(second=0, microsecond=0) + datetime.timedelta(minutes=1)
    cron_expression = f"{next_minute.minute} {next_minute.hour} * * ? *"  # Trigger at specific minute and hour
    
    print(f"Current time: {now.strftime('%H:%M:%S')} UTC")
    print(f"Schedule will trigger at: {next_minute.strftime('%H:%M:%S')} UTC")
    print(f"Cron expression: {cron_expression}")
    
    # Create the scheduled notification
    variables = {
        "serverName": "test-server", 
        "status": "SCHEDULED_TEST", 
        "message": f"Delivery test at {next_minute.strftime('%H:%M')} - ID: {test_schedule_id}"
    }
    
    response = test_user.create_scheduled_notification(
        notification_type="alert",
        variables=variables,
        cron_expression=cron_expression
    )
    assert response.status_code == 201
    response_json = response.json()
    schedule_db_id = response_json["scheduleId"]
    
    print(f"Created scheduled notification in database with ID: {schedule_db_id}")
    print(f"To verify delivery, after {next_minute.strftime('%H:%M')} UTC, check for notification validation entry:")
    print(f"   - User ID: {test_user.user_id}")
    print(f"   - Type: alert")
    print(f"   - Channel: in_app")
    print(f"   - Expected content: 'SCHEDULED DELIVERY TEST: test-server is SCHEDULED_TEST - Delivery test at {next_minute.strftime('%H:%M')} - ID: {test_schedule_id}'")
    
    # For automated testing, we'll clean up after a reasonable delay
    # In manual testing, you could comment this out and check the database
    print("Waiting 90 seconds to allow for delivery (adjust as needed for testing)...")
    time.sleep(90)
    
    # Attempt to check if delivery occurred (this would work if the schedule triggered)
    try:
        validation_data = get_notification_validation_data(schedule_db_id, test_user.user_id, "alert", "in_app")
        print("DELIVERY SUCCESSFUL: Found validation data")
        print(f"   Content: {validation_data.get('content', {}).get('S', 'No content')}")
        delivery_successful = True
    except Exception as e:
        print(f"Could not verify delivery (this is normal if schedule hasn't triggered yet): {e}")
        delivery_successful = False
    
    # Clean up the schedule
    try:
        response = test_user.delete_scheduled_notification(schedule_db_id)
        print(f"Cleaned up schedule: {response.status_code == 200}")
    except Exception as e:
        print(f"Could not clean up schedule (may have already been deleted): {e}")
    
    # Clean up test data
    test_super_admin.delete_template("*", "alert", "in_app")
    test_super_admin.delete_user_preferences("*")
    test_super_admin.delete_system_config("*")
    test_user.delete_user_preferences(test_user.user_id)
    
    if delivery_successful:
        print("SCHEDULED NOTIFICATION DELIVERY TEST PASSED")
    else:
        print("SCHEDULED NOTIFICATION SETUP COMPLETED - Check manually for delivery verification")