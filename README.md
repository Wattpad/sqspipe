# sqspipe

## Purpose

Move all the messages (present at the time you start the program) from one SQS queue to another, for example if you want to feed all of a dead letter queue back into the original queue for processing.

## Usage

With aws environment variables for an IAM user (or real user if you roll that way) so that the program will have access to the SQS queues:

`docker run --rm --ti -e AWS_REGION="$AWS_REGION" -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" -e AWS_SESSION_TOKEN="$AWS_SESSION_TOKEN" -e AWS_SECURITY_TOKEN="$AWS_SECURITY_TOKEN" 723255503624.dkr.ecr.us-east-1.amazonaws.com/sqspipe:latest -source https://sqs.us-east-1.amazonaws.com/123456789/source_queue -destination https://sqs.us-east-1.amazonaws.com/123456789/destination_queue -workers 20`

## Building

- `go build .` if you're allergic to make or want to use local everything
- `make image` same as above, but in a nice makefile

