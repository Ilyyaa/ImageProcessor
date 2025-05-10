package main

import (
	"net/http"

	_ "task-service/docs"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title			Task Service API
// @version		1.0
// @description	API for managing async computational tasks
// @host			localhost:8080
// @BasePath		/
func main() {
	storage := NewInMemoryStorage()

	r := chi.NewRouter()
	r.With(authMiddleware(storage)).Post("/task", CreateTaskHandler(storage))
	r.With(authMiddleware(storage)).Get("/status/{taskID}", GetStatusHandler(storage))
	r.With(authMiddleware(storage)).Get("/result/{taskID}", GetResultHandler(storage))

	r.Post("/register", RegisterUserHandler(storage))
	r.Post("/login", LoginUserHandler(storage))

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	http.ListenAndServe(":8080", r)
}
