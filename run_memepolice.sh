#!/bin/bash
cleanup() {
    echo "{\"msg\": \"wrapper process killed\"}" >> "/var/log/memepolice.log"
    pkill -P $$
    exit 0
}
trap cleanup SIGINT SIGTERM

source build_fpcalc.sh
export BOT_TOKEN=$(cat secret.txt)
go build -o memepolice .

while true; do
    ./memepolice &>> "/var/log/memepolice.log"
    export EXIT_CODE=$?
    echo "{\"exit_code\": $EXIT_CODE}" >> "/var/log/memepolice.log"
    sleep 1
done