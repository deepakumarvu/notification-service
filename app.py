#!/usr/bin/env python3
import os
from aws_cdk import App, Environment
from notification_service_stack import NotificationServiceStack

app = App()

# Get environment variables
account = os.environ.get('CDK_DEFAULT_ACCOUNT')
region = os.environ.get('CDK_DEFAULT_REGION', 'ap-south-1')
environment_name = os.environ.get('ENVIRONMENT', 'dev')

# Create the stack
NotificationServiceStack(
    app, 
    f"NotificationService-{environment_name}",
    env=Environment(account=account, region=region),
    environment_name=environment_name
)

app.synth() 