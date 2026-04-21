#!/bin/bash

start_or_run () {
    docker inspect peril_rabbitmq > /dev/null 2>&1

    if [ $? -eq 0 ]; then
        echo "Starting Peril RabbitMQ container..."
        docker start peril_rabbitmq
    else
        echo "Peril RabbitMQ container not found, creating a new one..."
        docker run -d --name peril_rabbitmq \
            -p 5672:5672 -p 15672:15672 \
            -e RABBITMQ_DEFAULT_USER=guest \
            -e RABBITMQ_DEFAULT_PASS=guest \
            rabbitmq:3.13-management
    fi
}

wait_for_rabbitmq () {
    echo "Waiting for RabbitMQ to become available..."
    until docker exec peril_rabbitmq rabbitmqctl status > /dev/null 2>&1; do
        sleep 1
    done
}

init_topology () {
    start_or_run
    wait_for_rabbitmq

    echo "Initializing RabbitMQ exchanges and queues..."

    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare exchange name=peril_direct type=direct durable=true
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare exchange name=peril_topic type=topic durable=true
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare exchange name=peril_dlx type=direct durable=true
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare queue name=peril_dlq durable=true
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare binding source=peril_dlx destination_type=queue destination=peril_dlq routing_key=peril_dlq
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare queue name=game_logs durable=true
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare binding source=peril_topic destination_type=queue destination=game_logs routing_key=game_logs.*

    echo "Seeding a dead-letter message into peril_dlq..."
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest declare queue name=peril_ttl_dlq durable=false auto_delete=true arguments='{"x-dead-letter-exchange":"peril_dlx","x-dead-letter-routing-key":"peril_dlq","x-message-ttl":500}'
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest publish exchange=amq.default routing_key=peril_ttl_dlq payload='{"source":"init","message":"dead-letter seed"}'
    sleep 3
    docker exec peril_rabbitmq rabbitmqadmin -u guest -p guest delete queue name=peril_ttl_dlq || true

    echo "RabbitMQ topology initialized successfully."
}

case "$1" in
    start)
        init_topology
        ;;
    init)
        init_topology
        ;;
    stop)
        echo "Stopping Peril RabbitMQ container..."
        docker stop peril_rabbitmq
        ;;
    logs)
        echo "Fetching logs for Peril RabbitMQ container..."
        docker logs -f peril_rabbitmq
        ;;
    *)
        echo "Usage: $0 {start|init|stop|logs}"
        exit 1
esac
