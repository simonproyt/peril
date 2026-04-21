# learn-pub-sub-starter (Peril)

This is the starter code used in Boot.dev's [Learn Pub/Sub](https://learn.boot.dev/learn-pub-sub) course.

## RabbitMQ setup

Start the RabbitMQ container with guest/guest login:

    ./rabbit.sh start

Initialize the required exchanges and queues:

    ./rabbit.sh init

You can then connect the server and client using `amqp://guest:guest@localhost:5672/`.
