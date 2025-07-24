import pytest
from testutils.test_user import User
import os
import json

with open('cdk-outputs.json', 'r') as f:
    data = json.load(f)
    
ENVIRONMENT = os.getenv("ENVIRONMENT")
REGION = data[f"NotificationService-{ENVIRONMENT}"]["Region"]
ACCOUNT_ID = data[f"NotificationService-{ENVIRONMENT}"]["AccountId"]
USER_POOL_ID = data[f"NotificationService-{ENVIRONMENT}"]["UserPoolId"]
USER_POOL_CLIENT_ID = data[f"NotificationService-{ENVIRONMENT}"]["UserPoolClientId"]
API_GATEWAY_URL = data[f"NotificationService-{ENVIRONMENT}"]["APIGatewayURL"]
    

@pytest.fixture(scope="session")
def test_super_admin():
    admin = User("super_admin2@company.com", "TestPassword10!", "super_admin", REGION, USER_POOL_ID, USER_POOL_CLIENT_ID, API_GATEWAY_URL)
    admin.create_user()
    admin.authenticate_user()
    yield admin

@pytest.fixture(scope="session")
def test_user():
    user = User("user1@company.com", "TestPassword10!", "user", REGION, USER_POOL_ID, USER_POOL_CLIENT_ID, API_GATEWAY_URL)
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