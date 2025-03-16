#!/bin/bash
set -e
export BOT_TOKEN=$(cat secret.txt)
go build -o memepolice .
./memepolice &>> "/var/log/memepolice.log"
export EXIT_CODE=$?
echo "{\"exit_code\": $EXIT_CODE}" >> "/var/log/memepolice.log"