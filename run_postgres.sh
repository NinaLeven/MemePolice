#!/bin/bash
sudo mkdir -p /var/lib/postgresql/data
sudo docker run -d \
  --name bktpostgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=postgres \
  -v /var/lib/postgresql/data:/var/lib/postgresql/data \
  -p 5432:5432 \
    bktpostgres