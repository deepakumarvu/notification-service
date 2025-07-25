import boto3
from datetime import datetime, timezone
import jwt
import requests
import logging
import json
from urllib.parse import quote

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

class User:
    def __init__(self, email, password, role, region, user_pool_id, user_pool_client_id, api_gateway_url, notification_queue_url):
        self.user_id = None
        self.email = email
        self.password = password
        self.role = role
        self.region = region
        self.cognito_client = boto3.client('cognito-idp', region_name=region)
        self.dynamodb_client = boto3.client('dynamodb', region_name=region)
        self.sqs_client = boto3.client('sqs', region_name=region)
        self.id_token = None
        self.access_token = None
        self.refresh_token = None
        self.user_pool_id = user_pool_id
        self.user_pool_client_id = user_pool_client_id
        self.api_gateway_url = api_gateway_url
        self.notification_queue_url = notification_queue_url

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
    
    def create_user_preferences(self, context, preferences=None, timezone=None, language=None):
        """Create user preferences"""
        body = {"context": context}
        if preferences:
            body["preferences"] = preferences
        if timezone:
            body["timezone"] = timezone
        if language:
            body["language"] = language
        return self.make_api_request("POST", "/preferences", body=body)
    
    def get_user_preferences(self, context):
        """Get user preferences by context"""
        return self.make_api_request("GET", f"/preferences?context={context}")
    
    def get_user_preferences_list(self, limit=None, next_token=None):
        """List all user preferences (super admin only)"""
        query_params = []
        if limit:
            query_params.append(f"limit={limit}")
        if next_token:
            query_params.append(f"nextToken={next_token}")
        
        query_string = "&".join(query_params)
        path = "/preferences"
        if query_string:
            path += f"?{query_string}"
        
        return self.make_api_request("GET", path)
    
    def update_user_preferences(self, context, preferences=None, timezone=None, language=None):
        """Update user preferences"""
        body = {"context": context}
        if preferences is not None:
            body["preferences"] = preferences
        if timezone is not None:
            body["timezone"] = timezone
        if language is not None:
            body["language"] = language
        return self.make_api_request("PUT", "/preferences", body=body)
    
    def delete_user_preferences(self, context):
        """Delete user preferences by context"""
        return self.make_api_request("DELETE", f"/preferences?context={context}")
    
    def create_system_config(self, context, config=None, description=None):
        """Create system config"""
        body = {"context": context}
        if config:
            body["config"] = config
        if description:
            body["description"] = description
        return self.make_api_request("POST", "/config", body=body)
    
    def get_system_config(self, context):
        """Get system config by context"""
        return self.make_api_request("GET", f"/config?context={context}")
    
    def get_system_config_list(self, limit=None, next_token=None):
        """List all system configs (super admin only)"""
        query_params = []
        if limit:
            query_params.append(f"limit={limit}")
        if next_token:
            query_params.append(f"nextToken={next_token}")
        
        query_string = "&".join(query_params)
        path = "/config"
        if query_string:
            path += f"?{query_string}"
        
        return self.make_api_request("GET", path)
    
    def update_system_config(self, context, config=None, description=None):
        """Update system config"""
        body = {"context": context}
        if config is not None:
            body["config"] = config
        if description is not None:
            body["description"] = description
        return self.make_api_request("PUT", "/config", body=body)
    
    def delete_system_config(self, context):
        """Delete system config by context"""
        return self.make_api_request("DELETE", f"/config?context={context}")
    
    def send_notification_to_queue(self, id, notification_type, recipients, variables=None):
        """Send a notification request to SQS queue"""
        if variables is None:
            variables = {}
            
        message_body = {
            "id": id,
            "type": notification_type,
            "recipients": recipients if isinstance(recipients, list) else [recipients],
            "variables": variables
        }
        
        try:
            response = self.sqs_client.send_message(
                QueueUrl=self.notification_queue_url,
                MessageBody=json.dumps(message_body)
            )
            logger.info(f"Sent {notification_type} notification to queue. MessageId: {response['MessageId']}")
            return response
        except Exception as e:
            logger.error(f"Failed to send notification to queue: {e}")
            raise
    
    def send_alert_notification(self, id, recipients, server_name, environment, status="critical", message="System alert"):
        """Send an alert notification"""
        variables = {
            "serverName": server_name,
            "environment": environment,
            "status": status,
            "message": message
        }
        return self.send_notification_to_queue(id, "alert", recipients, variables)
    
    def send_report_notification(self, id, recipients, report_name, time_period, summary="Report generated"):
        """Send a report notification"""
        variables = {
            "reportType": report_name,
            "period": time_period,
            "data": summary,
        }
        return self.send_notification_to_queue(id, "report", recipients, variables)
    
    def send_general_notification(self, id, recipients, title, message, action_url=None):
        """Send a general notification"""
        variables = {
            "title": title,
            "message": message,
            "actionUrl": action_url
        }
        return self.send_notification_to_queue(id, "notification", recipients, variables)
    