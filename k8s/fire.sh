#!/bin/bash

{
    sleep 5m
    kill $$
} &

docker-compose -f docker-compose.yml -p elastic-package-service up --build  --force-recreate --renew-anon-volumes
