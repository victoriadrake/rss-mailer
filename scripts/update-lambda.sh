#!/bin/bash

set -euxo pipefail
source .env
GOOS=linux
BUILD_NAME=${FUNCTION_NAME:-"rss-mailer"}

go build && zip "$BUILD_NAME".zip rss-mailer template.html

aws lambda update-function-code \
    --function-name "$BUILD_NAME" \
    --zip-file fileb://"$BUILD_NAME".zip

rm rss-mailer "$BUILD_NAME".zip

aws lambda update-function-configuration \
    --function-name "$BUILD_NAME" \
    --environment "$LAMBDA_ENV"
