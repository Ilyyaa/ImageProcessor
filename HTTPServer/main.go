package main

import (
	"net/http"

	_ "task-service/docs"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"

	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

// @title			Code proccesor
// @version		1.0
// @description	API for managing async computational tasks
// @host			localhost:8080
// @BasePath		/
func main() {
	storage := NewInMemoryStorage()

	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	r := chi.NewRouter()
	r.With(authMiddleware(storage)).Post("/task", CreateTaskHandler(ch, storage))
	r.With(authMiddleware(storage)).Get("/status/{taskID}", GetStatusHandler(ch, storage))
	r.With(authMiddleware(storage)).Get("/result/{taskID}", GetResultHandler(ch, storage))

	r.Post("/register", RegisterUserHandler(storage))
	r.Post("/login", LoginUserHandler(storage))

	r.Post("/Commit", CommitHandler(storage))

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	http.ListenAndServe("0.0.0.0:8080", r)
}
