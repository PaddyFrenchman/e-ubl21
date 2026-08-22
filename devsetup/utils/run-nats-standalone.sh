#!/usr/bin/env bash
docker run -d --name nats -p 4222:4222 -p 8222:8222 -v nats-data:/data nats:2.14-alpine -js -sd /data -m 8222