#!/bin/bash

# Parse common arguments
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        --region)
            export AWS_REGION="$2"
            shift
            shift
            ;;
        --profile)
            export AWS_PROFILE="$2"
            shift
            shift
            ;;
        *)
            command=$1
            shift #For command parameters, these will be parsed later
            ;;
    esac
done

# Parse command parameters
if [ "$command" == "--setup" ]; then
    # Bootstrap with a custom qualifier. This also configures the CDK toolkit stack.
    cdk bootstrap
    exit 0
elif [ "$command" == "--diff" ]; then
    # Use context to point to the custom asset bucket
    cdk diff
    exit 0
elif [ "$command" == "--destroy" ]; then
    # Destroy the stack
    cdk destroy
    exit 0
elif [ "$command" == "--synth" ]; then
    cdk synth > cdk-output.yaml
    exit 0
fi

cdk deploy --require-approval never --asset-parallelism true --outputs-file cdk-outputs.json
