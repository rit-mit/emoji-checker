#! /bin/bash

PARAMS="$@"

docker run --rm -v "$PWD":/go/src/handler lambci/lambda:build-go1.x sh -c 'go mod download && go build main.go' && docker run --rm -v "$PWD":/var/task lambci/lambda:go1.x main "$PARAMS"
