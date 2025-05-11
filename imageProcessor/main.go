package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"log"
	"strings"

	"net/http"

	imaging "github.com/disintegration/imaging"
	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

type ImageFilterMessage struct {
	TaskId      string `json:"taskId"`
	ImageBase64 string `json:"imageBase64"`
	FilterName  string `json:"filterName"`
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"code", // name
		false,  // durable
		false,  // delete when unused
		false,  // exclusive
		false,  // no-wait
		nil,    // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			//Encode the data
			var data ImageFilterMessage
			err := json.Unmarshal(d.Body, &data)
			log.Println(string(d.Body))
			if err != nil {
				log.Fatalf("An Error Occured %v", err)
			}

			commaIndex := strings.Index(data.ImageBase64, ",")
			if commaIndex != -1 {
				data.ImageBase64 = data.ImageBase64[commaIndex+1:]
			}

			imgData, err := base64.StdEncoding.DecodeString(data.ImageBase64)
			if err != nil {
				log.Fatalf("failed to decode base64 string: %v", err)
			}

			img, _, err := image.Decode(bytes.NewReader(imgData))
			if err != nil {
				log.Fatalf("failed to decode image: %v", err)
			}

			var result *image.NRGBA
			if data.FilterName == "blur" {
				result = imaging.Blur(img, 30)
			}

			var buf bytes.Buffer
			err = png.Encode(&buf, result)
			if err != nil {
				log.Fatalf("Ошибка кодирования PNG: %v", err)
			}
			encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

			log.Printf(" [*] consumed %s\n", string(d.Body))
			postBody, _ := json.Marshal(map[string]string{
				"id":     data.TaskId,
				"result": encoded,
				"status": "ready",
			})

			responseBody := bytes.NewBuffer(postBody)
			resp, err := http.Post("http://publisher:8080/Commit", "application/json", responseBody)
			log.Printf(" [*] sent post request\n")
			if err != nil {
				log.Fatalf("An Error Occured %v", err)
			}
			defer resp.Body.Close()
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
