from aws_cdk import (
    Stack,
    CfnOutput,
    Duration,
    RemovalPolicy,
    aws_dynamodb as dynamodb,
    aws_lambda as _lambda,
    aws_lambda_event_sources as lambda_event_sources,
    aws_apigateway as apigateway,
    aws_cognito as cognito,
    aws_iam as iam,
    aws_logs as logs,
)
from constructs import Construct
import os
import time

class NotificationServiceStack(Stack):

    def __init__(self, scope: Construct, construct_id: str, environment_name: str = "dev", **kwargs) -> None:
        super().__init__(scope, construct_id, **kwargs)
        
        self.environment_name = environment_name
        
        # Create DynamoDB tables
        self._create_dynamodb_tables()
        
        # Create Cognito User Pool
        self._create_cognito_user_pool()
        
        # Create SQS Queue
        self._create_sqs_queue()
        
        # Create Lambda functions
        self._create_lambda_functions()
        
        # Create API Gateway
        self._create_api_gateway()
        
        # Create outputs
        self._create_outputs()

    def _create_dynamodb_tables(self):
        """Create DynamoDB tables for the notification service"""
        
        # Users table
        self.users_table = dynamodb.Table(
            self, f"Users-{self.environment_name}",
            table_name=f"notification-service-users-{self.environment_name}",
            partition_key=dynamodb.Attribute(
                name="userId",
                type=dynamodb.AttributeType.STRING
            ),
            billing_mode=dynamodb.BillingMode.PAY_PER_REQUEST,
            encryption=dynamodb.TableEncryption.AWS_MANAGED,
            point_in_time_recovery=True,
            removal_policy=RemovalPolicy.DESTROY if self.environment_name == "dev" else RemovalPolicy.RETAIN
        )
        
        # Add GSI for email lookup
        self.users_table.add_global_secondary_index(
            index_name="EmailIndex",
            partition_key=dynamodb.Attribute(
                name="email",
                type=dynamodb.AttributeType.STRING
            ),
            projection_type=dynamodb.ProjectionType.ALL
        )
        
        # Templates table
        self.templates_table = dynamodb.Table(
            self, f"Templates-{self.environment_name}",
            table_name=f"notification-service-templates-{self.environment_name}",
            partition_key=dynamodb.Attribute(
                name="context",
                type=dynamodb.AttributeType.STRING
            ),
            sort_key=dynamodb.Attribute(
                name="type#channel",
                type=dynamodb.AttributeType.STRING
            ),
            billing_mode=dynamodb.BillingMode.PAY_PER_REQUEST,
            encryption=dynamodb.TableEncryption.AWS_MANAGED,
            point_in_time_recovery=True,
            removal_policy=RemovalPolicy.DESTROY if self.environment_name == "dev" else RemovalPolicy.RETAIN
        )
        
        # User Preferences table
        self.preferences_table = dynamodb.Table(
            self, f"Preferences-{self.environment_name}",
            table_name=f"notification-service-preferences-{self.environment_name}",
            partition_key=dynamodb.Attribute(
                name="context",
                type=dynamodb.AttributeType.STRING
            ),
            billing_mode=dynamodb.BillingMode.PAY_PER_REQUEST,
            encryption=dynamodb.TableEncryption.AWS_MANAGED,
            point_in_time_recovery=True,
            removal_policy=RemovalPolicy.DESTROY if self.environment_name == "dev" else RemovalPolicy.RETAIN
        )
        
        # System Configuration table
        self.config_table = dynamodb.Table(
            self, f"Config-{self.environment_name}",
            table_name=f"notification-service-config-{self.environment_name}",
            partition_key=dynamodb.Attribute(
                name="context",
                type=dynamodb.AttributeType.STRING
            ),
            billing_mode=dynamodb.BillingMode.PAY_PER_REQUEST,
            encryption=dynamodb.TableEncryption.AWS_MANAGED,
            point_in_time_recovery=True,
            removal_policy=RemovalPolicy.DESTROY if self.environment_name == "dev" else RemovalPolicy.RETAIN
        )
        
    def _create_cognito_user_pool(self):
        """Create Cognito User Pool for authentication"""
        
        self.user_pool = cognito.UserPool(
            self, f"UserPool-{self.environment_name}",
            user_pool_name=f"notification-service-{self.environment_name}",
            sign_in_aliases=cognito.SignInAliases(email=True),
            auto_verify=cognito.AutoVerifiedAttrs(email=True),
            password_policy=cognito.PasswordPolicy(
                min_length=8,
                require_lowercase=True,
                require_uppercase=True,
                require_digits=True,
                require_symbols=True
            ),
            custom_attributes={
                "role": cognito.StringAttribute(mutable=True)
            },
            removal_policy=RemovalPolicy.DESTROY if self.environment_name == "dev" else RemovalPolicy.RETAIN
        )
        
        self.user_pool_client = cognito.UserPoolClient(
            self, f"UserPoolClient-{self.environment_name}",
            user_pool=self.user_pool,
            auth_flows=cognito.AuthFlow(
                user_password=True,
                user_srp=True,
                admin_user_password=True
            ),
            generate_secret=False,
            access_token_validity=Duration.hours(1),
            refresh_token_validity=Duration.days(30)
        )

    def _create_sqs_queue(self):
        """Create SQS queue for notification processing"""
        
        # Dead letter queue
        self.dlq = sqs.Queue(
            self, f"NotificationDLQ-{self.environment_name}",
            queue_name=f"notification-service-dlq-{self.environment_name}",
            retention_period=Duration.days(14)
        )
        
        # Main notification queue
        self.notification_queue = sqs.Queue(
            self, f"NotificationQueue-{self.environment_name}",
            queue_name=f"notification-service-queue-{self.environment_name}",
            visibility_timeout=Duration.minutes(5),
            dead_letter_queue=sqs.DeadLetterQueue(
                max_receive_count=3,
                queue=self.dlq
            )
        )

    def _create_lambda_functions(self):
        """Create Lambda functions for the notification service"""
        
        # Common Lambda configuration
        lambda_environment = {
            "USERS_TABLE": self.users_table.table_name,
            "TEMPLATES_TABLE": self.templates_table.table_name,
            "PREFERENCES_TABLE": self.preferences_table.table_name,
            "CONFIG_TABLE": self.config_table.table_name,
            "NOTIFICATION_QUEUE_URL": self.notification_queue.queue_url,
            "NOTIFICATION_TOPIC_ARN": self.notification_topic.topic_arn,
            "USER_POOL_ID": self.user_pool.user_pool_id,
            "ENVIRONMENT": self.environment_name,
            "REGION": self.region
        }
        
        # Lambda execution role
        lambda_role = iam.Role(
            self, f"LambdaExecutionRole-{self.environment_name}",
            assumed_by=iam.ServicePrincipal("lambda.amazonaws.com"),
            managed_policies=[
                iam.ManagedPolicy.from_aws_managed_policy_name("service-role/AWSLambdaBasicExecutionRole")
            ]
        )
        
        # Grant permissions to DynamoDB tables
        self.users_table.grant_read_write_data(lambda_role)
        self.templates_table.grant_read_write_data(lambda_role)
        self.preferences_table.grant_read_write_data(lambda_role)
        self.config_table.grant_read_write_data(lambda_role)
        
        # Grant permissions to Cognito
        lambda_role.add_to_policy(
            iam.PolicyStatement(
                actions=["cognito-idp:*"],
                resources=[self.user_pool.user_pool_arn]
            )
        )
        
        # User Handler Lambda
        self.user_handler = _lambda.Function(
            self, f"UserHandler-{self.environment_name}",
            function_name=f"NotificationService-UserHandler-{self.environment_name}",
            runtime=_lambda.Runtime.PROVIDED_AL2,
            handler="bootstrap",
            code=_lambda.Code.from_asset("./build/user"),
            environment=lambda_environment,
            role=lambda_role,
            timeout=Duration.seconds(30),
            memory_size=256,
            log_retention=logs.RetentionDays.ONE_WEEK
        )

        # Template Handler Lambda
        self.template_handler = _lambda.Function(
            self, f"TemplateHandler-{self.environment_name}",
            function_name=f"NotificationService-TemplateHandler-{self.environment_name}",
            runtime=_lambda.Runtime.PROVIDED_AL2,
            handler="bootstrap",
            code=_lambda.Code.from_asset("./build/template"),
            environment=lambda_environment,
            role=lambda_role,
            timeout=Duration.seconds(30),
            memory_size=256,
            log_retention=logs.RetentionDays.ONE_WEEK
        )

        # Preference Handler Lambda
        self.preference_handler = _lambda.Function(
            self, f"PreferenceHandler-{self.environment_name}",
            function_name=f"NotificationService-PreferenceHandler-{self.environment_name}",
            runtime=_lambda.Runtime.PROVIDED_AL2,
            handler="bootstrap",
            code=_lambda.Code.from_asset("./build/preference"),
            environment=lambda_environment,
            role=lambda_role,
            timeout=Duration.seconds(30),
            memory_size=256,
            log_retention=logs.RetentionDays.ONE_WEEK
        )

        # Config Handler Lambda
        self.config_handler = _lambda.Function(
            self, f"ConfigHandler-{self.environment_name}",
            function_name=f"NotificationService-ConfigHandler-{self.environment_name}",
            runtime=_lambda.Runtime.PROVIDED_AL2,
            handler="bootstrap",
            code=_lambda.Code.from_asset("./build/config"),
            environment=lambda_environment,
            role=lambda_role,
            timeout=Duration.seconds(30),
            memory_size=256,
            log_retention=logs.RetentionDays.ONE_WEEK
        )

        # Notification Processor Lambda
        self.processor_handler = _lambda.Function(
            self, f"ProcessorHandler-{self.environment_name}",
            function_name=f"NotificationService-ProcessorHandler-{self.environment_name}",
            runtime=_lambda.Runtime.PROVIDED_AL2,
            handler="bootstrap",
            code=_lambda.Code.from_asset("./build/processor"),
            environment=lambda_environment,
            role=lambda_role,
            timeout=Duration.seconds(60),  # Longer timeout for processing
            memory_size=512,               # More memory for processing multiple recipients
            log_retention=logs.RetentionDays.ONE_WEEK
        )

        # Grant SQS permissions to the processor
        self.notification_queue.grant_consume_messages(lambda_role)

        # Add SQS event source to trigger the processor
        self.processor_handler.add_event_source(
            lambda_event_sources.SqsEventSource(
                self.notification_queue,
                batch_size=10,  # Process up to 10 messages at once
                report_batch_item_failures=True
            )
        )

    def _create_api_gateway(self):
        """Create API Gateway for the REST API"""
        
        # Lambda Authorizer
        self.authorizer = apigateway.CognitoUserPoolsAuthorizer(
            self, f"CognitoAuthorizer-{self.environment_name}",
            cognito_user_pools=[self.user_pool]
        )
        
        # API Gateway
        self.api = apigateway.RestApi(
            self, f"NotificationServiceAPI-{self.environment_name}",
            rest_api_name=f"notification-service-{self.environment_name}",
            description=f"Notification Service API - {self.environment_name}",
            default_cors_preflight_options=apigateway.CorsOptions(
                allow_origins=apigateway.Cors.ALL_ORIGINS,
                allow_methods=apigateway.Cors.ALL_METHODS,
                allow_headers=["Content-Type", "X-Amz-Date", "Authorization", "X-Api-Key"]
            ),
            default_method_options=apigateway.MethodOptions(
                authorization_type=apigateway.AuthorizationType.COGNITO,
                authorizer=self.authorizer
            )
        )
        
        # API versions
        api_v1 = self.api.root.add_resource("api").add_resource("v1")
        
        # Users endpoints
        users_resource = api_v1.add_resource("users")
        user_resource = users_resource.add_resource("{userId}")
        
        users_resource.add_method(
            "GET", 
            apigateway.LambdaIntegration(self.user_handler),
        )
        user_resource.add_method(
            "GET", 
            apigateway.LambdaIntegration(self.user_handler),
        )
        
        # Templates endpoints
        templates_resource = api_v1.add_resource("templates")
        template_resource = templates_resource.add_resource("{templateId}")
        
        templates_resource.add_method(
            "GET", 
            apigateway.LambdaIntegration(self.template_handler),
        )
        templates_resource.add_method(
            "POST", 
            apigateway.LambdaIntegration(self.template_handler),
        )
        template_resource.add_method(
            "GET", 
            apigateway.LambdaIntegration(self.template_handler),
        )
        template_resource.add_method(
            "PUT", 
            apigateway.LambdaIntegration(self.template_handler),
        )
        template_resource.add_method(
            "DELETE", 
            apigateway.LambdaIntegration(self.template_handler),
        )
        
        # Preferences endpoints
        preferences_resource = api_v1.add_resource("preferences")
        
        preferences_resource.add_method(
            "GET", 
            apigateway.LambdaIntegration(self.preference_handler),
        )
        preferences_resource.add_method(
            "POST", 
            apigateway.LambdaIntegration(self.preference_handler),
        )
        preferences_resource.add_method(
            "PUT", 
            apigateway.LambdaIntegration(self.preference_handler),
        )
        preferences_resource.add_method(
            "DELETE", 
            apigateway.LambdaIntegration(self.preference_handler),
        )
        
        # Config endpoints
        config_resource = api_v1.add_resource("config")
        
        config_resource.add_method(
            "GET", 
            apigateway.LambdaIntegration(self.config_handler),
        )
        config_resource.add_method(
            "POST", 
            apigateway.LambdaIntegration(self.config_handler),
        )
        config_resource.add_method(
            "PUT", 
            apigateway.LambdaIntegration(self.config_handler),
        )
        config_resource.add_method(
            "DELETE", 
            apigateway.LambdaIntegration(self.config_handler),
        )
        

    def _create_outputs(self):
        """Create CloudFormation outputs"""
        
        CfnOutput(
            self, "APIGatewayURL",
            value=self.api.url,
            description="API Gateway URL"
        )
        
        CfnOutput(
            self, "UserPoolId",
            value=self.user_pool.user_pool_id,
            description="Cognito User Pool ID"
        )
        
        CfnOutput(
            self, "UserPoolClientId",
            value=self.user_pool_client.user_pool_client_id,
            description="Cognito User Pool Client ID"
        )

        CfnOutput(
            self, "Region",
            value=self.region,
            description="AWS Region"
        )
        
        CfnOutput(
            self, "AccountId",
            value=self.account,
            description="AWS Account ID"
        )

        CfnOutput(
            self, "NotificationQueueURL",
            value=self.notification_queue.queue_url,
            description="Notification Queue URL"
        )

        CfnOutput(
            self, "NotificationQueueARN",
            value=self.notification_queue.queue_arn,
            description="Notification Queue ARN"
        )