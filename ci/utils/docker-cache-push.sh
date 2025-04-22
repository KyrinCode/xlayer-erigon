#!/bin/bash

IMAGES=$(cat docker-images.txt | tr '\n' ' ')

docker run -d -p 5000:5000 --name registry registry:latest
docker exec registry sh -c "ifconfig eth0 | grep 'inet addr' | tr -s ' ' | cut -d ' ' -f 3 | cut -d ':' -f 2"

for IMAGE in $IMAGES; do
    docker pull $IMAGE
    docker tag $IMAGE localhost:5000/$IMAGE
    docker push localhost:5000/$IMAGE
    docker rmi localhost:5000/$IMAGE
done