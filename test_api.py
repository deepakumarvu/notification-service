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
    
    