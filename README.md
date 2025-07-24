# Notification Service

A scalable, multi-channel notification service built on AWS that supports email, Slack, and in-app notifications with flexible scheduling and templating capabilities.

## Prerequisites Install

```sh
pip install -r requirements.txt
```

1. Also setup the direnv to load the environment variables from the .envrc file

## Deploy
```sh
make deploy
```

## Test
```sh
pytest test_api.py -v -s
```
