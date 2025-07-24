import boto3
from datetime import datetime, timezone
import jwt
import requests
import logging
from urllib.parse import quote

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

class User:
    def __init__(self, email, password, role, region, user_pool_id, user_pool_client_id, api_gateway_url):
        self.user_id = None
        self.email = email
        self.password = password
        self.role = role
        self.cognito_client = boto3.client('cognito-idp', region_name=region)
        self.dynamodb_client = boto3.client('dynamodb', region_name=region)
        self.id_token = None
        self.access_token = None
        self.refresh_token = None
        self.user_pool_id = user_pool_id
        self.user_pool_client_id = user_pool_client_id
        self.api_gateway_url = api_gateway_url

    def __str__(self):
        return f"User(user_id={self.user_id}, email={self.email}, role={self.role})"
    
    def create_user(self):
        """Create a user in Cognito and DynamoDB"""
        
        # Create user in Cognito
        try:
            cognito_response = self.cognito_client.admin_create_user(
                UserPoolId=self.user_pool_id,
                Username=self.email,
                UserAttributes=[
                    {'Name': 'email', 'Value': self.email},
                    {'Name': 'custom:role', 'Value': self.role}
                ],
                TemporaryPassword=self.password,
                MessageAction='SUPPRESS'  # Don't send welcome email in tests
            )
            
            if cognito_response['ResponseMetadata']['HTTPStatusCode'] != 200:
                raise Exception(f"Failed to create Cognito user: {cognito_response}")
            
            # Set permanent password
            self.cognito_client.admin_set_user_password(
                UserPoolId=self.user_pool_id,
                Username=self.email,
                Password=self.password,
                Permanent=True
            )
            
            cognito_user_id = self.cognito_client.admin_get_user(
                UserPoolId=self.user_pool_id,
                Username=self.email
            )
            
            user_id = cognito_user_id["Username"]

            # Directly create users in DynamoDB
            self.dynamodb_client.put_item(
                TableName="notification-service-users-dev",
                Item={
                    "userId": {"S": user_id},
                    "email": {"S": self.email},
                    "role": {"S": self.role},
                    "isActive": {"BOOL": True},
                    "createdAt": {"S": datetime.now(timezone.utc).isoformat()},
                    "updatedAt": {"S": datetime.now(timezone.utc).isoformat()},
                }
            )
            return user_id
        except self.cognito_client.exceptions.UsernameExistsException:
            logger.warning(f"User {self.email} already exists")
            return False
        except Exception as e:
            logger.error(f"Error creating Cognito user: {e}")
            raise
        
    def authenticate_user(self) -> bool:
        """Authenticate user and get tokens"""
        try:
            # Use Cognito authentication directly
            response = self.cognito_client.admin_initiate_auth(
                UserPoolId=self.user_pool_id,
                ClientId=self.user_pool_client_id,
                AuthFlow='ADMIN_NO_SRP_AUTH',
                AuthParameters={
                    'USERNAME': self.email,
                    'PASSWORD': self.password
                }
            )
            
            tokens = response['AuthenticationResult']
            self.id_token = tokens['IdToken']
            self.access_token = tokens['AccessToken']
            self.refresh_token = tokens['RefreshToken']
            
            # Decode the access token to get the user ID
            decoded_token = jwt.decode(self.access_token, options={"verify_signature": False})
            self.user_id = decoded_token['sub']
            
            logger.info(f"Authentication successful for user {self.email}")
            return True
            
        except Exception as e:
            logger.error(f"Authentication failed: {e}")
            return False
        
    def make_api_request(self, method, path, body=None):
        headers = {
            "Authorization": f"{self.id_token}",
            "Content-Type": "application/json"
        }
        # Print Request
        logger.info(f"Making {method} request to {self.api_gateway_url}api/v1{path}, headers: {headers}, body: {body}")
        response = requests.request(method, f"{self.api_gateway_url}api/v1{path}", headers=headers, json=body)
        logger.info(f"Response: {response.text}")
        return response
    
    def get_users_list(self):
        return self.make_api_request("GET", "/users")
    
    def get_user_by_id(self, user_id):
        return self.make_api_request("GET", f"/users/{user_id}")
    
    def create_template(self, context, type, channel, content):
        return self.make_api_request("POST", "/templates", body={
            "context": context,
            "type": type,
            "channel": channel,
            "content": content
        })
    
    def get_templates_list(self, context):
        return self.make_api_request("GET", f"/templates?context={context}")
    
    def get_template_by_id(self, context, type, channel):
        encoded_type_channel = quote(f"{type}#{channel}", safe='')
        return self.make_api_request("GET", f"/templates/{encoded_type_channel}?context={context}")
    
    def update_template(self, context, type, channel, content):
        encoded_type_channel = quote(f"{type}#{channel}", safe='')
        return self.make_api_request("PUT", f"/templates/{encoded_type_channel}", body={"context": context, "type": type, "channel": channel, "content": content})
    
    def delete_template(self, context, type, channel):
        encoded_type_channel = quote(f"{type}#{channel}", safe='')
        return self.make_api_request("DELETE", f"/templates/{encoded_type_channel}?context={context}")
    