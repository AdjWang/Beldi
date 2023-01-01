#!/bin/bash
set -euxo pipefail

SCRIPT_PATH=$(readlink -f $0)
BASE_DIR=$(dirname $SCRIPT_PATH)
PROJECT_DIR=$(dirname $BASE_DIR)

HANDLER_DIR=$PROJECT_DIR/cmd/handler/hotel

for dir in flight frontend gateway geo hotel order profile rate recommendation search user collector; do
    cd $HANDLER_DIR/$dir
    go build handler.go
    cd -
done
