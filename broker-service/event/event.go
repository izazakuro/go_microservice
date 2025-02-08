package event

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

func declareExchange(ch *amqp.Channel) error {
	return ch.ExchangeDeclare(
		"logs_topic", // name
		"topic",      // type
		true,         // duable?
		false,        // autoDelete?
		false,        // internal?
		false,        // no-wait?
		nil,          // arguements
	)
}

func declareRandomQueue(ch *amqp.Channel) (amqp.Queue, error) {
	return ch.QueueDeclare(
		"",    //name
		false, //durable?
		false, //autodelete?
		true,  //exclusive?
		false, //no-wait
		nil,   //arguements?
	)
}
