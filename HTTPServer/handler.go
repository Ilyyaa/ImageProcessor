package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"golang.org/x/crypto/bcrypt"
)

type TaskResponse struct {
	TaskID string `json:"task_id"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type ImageFilterMessage struct {
	TaskId      string `json:"taskId"`
	ImageBase64 string `json:"imageBase64"`
	FilterName  string `json:"filterName"`
}

type ImageRequest struct {
	ImageBase64 string `json:"imageBase64" example:"/9j/4AAQSk..."`
	FilterName  string `json:"filterName" example:"Sepia"`
}

// @Summary Submit a new task
// @Accept multipart/form-data
// @Produce json
// @Param Authorization header string true "Auth token"
// @Param image formData file true "Image file"
// @Param filtername formData string true "Name of the filter"
// @Success 200 {object} TaskResponse
// @Failure 401 {string} string "Invalid token"
// @Router /task [post]
func CreateTaskHandler(ch *amqp.Channel, store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		failOnError(err, "Failed to parse multipart form")

		file, _, err := r.FormFile("image")
		failOnError(err, "Failed to get image file")
		defer file.Close()

		imageBytes, err := io.ReadAll(file)
		failOnError(err, "Failed to read image file")

		imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)

		filterName := r.FormValue("filtername")

		taskID := uuid.New().String()
		store.SetTask(taskID, Task{Status: "in_progress"})

		message := ImageFilterMessage{
			TaskId:      taskID,
			ImageBase64: imageBase64,
			FilterName:  filterName,
		}

		messageBytes, err := json.Marshal(message)
		failOnError(err, "Failed to marshal message to JSON")

		q, err := ch.QueueDeclare(
			"code", // name
			false,  // durable
			false,  // delete when unused
			false,  // exclusive
			false,  // no-wait
			nil,    // arguments
		)
		failOnError(err, "Failed to declare a queue")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = ch.PublishWithContext(
			ctx,
			"",     // exchange
			q.Name, // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        messageBytes,
			})
		failOnError(err, "Failed to publish a message")
		log.Printf(" [x] Sent %s\n", message.TaskId)

		json.NewEncoder(w).Encode(TaskResponse{TaskID: taskID})
	}
}

// @Summary Get task status
// @Produce json
// @Param taskID path string true "Task ID"
// @Param Authorization header string true "Auth token"
// @Success 200 {object} StatusResponse
// @Failure 401 {string} string "Invalid token"
// @Router /status/{taskID} [get]
func GetStatusHandler(ch *amqp.Channel, store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := chi.URLParam(r, "taskID")
		task, ok := store.GetTask(taskID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(StatusResponse{Status: task.Status})
	}
}

// @Summary Get task result
// @Produce plain
// @Param taskID path string true "Task ID"
// @Param Authorization header string true "Auth token"
// @Success 200 {string} string "result"
// @Failure 404 {string} string "not found"
// @Failure 401 {string} string "Invalid token"
// @Router /result/{taskID} [get]
func GetResultHandler(ch *amqp.Channel, store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := chi.URLParam(r, "taskID")
		task, ok := store.GetTask(taskID)
		if !ok || task.Status != "ready" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(task.Result))
	}
}

type AuthUserRequest struct {
	Username string `json:"username" example:"johndoe"`
	Password string `json:"password" example:"securePassword123"`
}

// @Summary Register user
// @Description  Accepts a JSON object with username and password to create a new user
// @tags auth
// @Accept json
// @Produce plain
// @Param user body AuthUserRequest true "User data"
// @Success      201   {string}  string  "User successfully registered"
// @Failure      400   {string}  string  "Invalid input or missing fields"
// @Failure      500   {string}  string  "Internal server error"
// @Router       /register [post]
func RegisterUserHandler(store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := uuid.New().String()
		var data AuthUserRequest
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		username := data.Username
		password := data.Password

		if username == "" || password == "" {
			http.Error(w, "Missing username or password", http.StatusBadRequest)
		}

		if err := store.RegisterUser(userID, username, password); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

	}
}

// @Summary Login user
// @tags auth
// @Accept json
// @Produce plain
// @Param user body AuthUserRequest true "User data"
// @Success      200   {string}  string  "User successfully logined"
// @Failure      401   {string}  string  "Invalid username or password"
// @Router       /login [post]
func LoginUserHandler(store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data AuthUserRequest
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		SessionId := uuid.New().String()
		User, exists := store.GetUserByLogin(data.Username)

		if !exists || bcrypt.CompareHashAndPassword([]byte(User.hash), []byte(data.Password)) != nil {
			http.Error(w, "Invalid username or password", http.StatusBadRequest)
			return
		}

		Session := Session{UserId: User.id,
			SessionId: SessionId}
		store.SetSession(Session)

		w.Header().Add("Authorization", SessionId)
		w.WriteHeader(http.StatusOK)
	}
}

func authMiddleware(store Storage) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if token == "" {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
			if len(token) <= len("Bearer ") {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
			_, exists := store.GetSession(token[len("Bearer "):])
			if !exists {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type CommitRequest struct {
	Id     string
	Result string
	Status string
}

func CommitHandler(store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data CommitRequest
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		store.SetTask(data.Id, Task{Status: data.Status, Result: data.Result})
		log.Printf("%s %s %s", data.Id, data.Status, data.Result)
		w.WriteHeader(http.StatusOK)
	}
}
