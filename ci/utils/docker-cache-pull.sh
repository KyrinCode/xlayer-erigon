#!/bin/bash

IMAGES=$(cat docker-images.txt | tr '\n' ' ')

for IMAGE in $IMAGES; do
    docker pull 172.17.0.2:5000/$IMAGE
    docker tag 172.17.0.2:5000/$IMAGE $IMAGE
done