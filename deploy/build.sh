#!/bin/bash

set -e
set -x

godep go build -a github.com/Clever/cadvisor

docker build -t clever/cadvisor:$(git rev-parse --short HEAD) .
