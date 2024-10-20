#!/bin/bash
rm -rf /tmp/bktree
mkdir /tmp/bktree
git clone 'https://github.com/fake-name/pg-spgist_hamming.git' /tmp/bktree
cp PostgresDockerfile /tmp/bktree/Dockerfile
cd /tmp/bktree
sudo docker build -t bktpostgres . 
rm /tmp/bktree/Dockerfile