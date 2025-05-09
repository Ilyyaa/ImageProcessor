package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type TaskResponse struct {
	TaskID string `json:"task_id"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

// @Summary Submit a new task
// @Produce json
// @Success 200 {object} TaskResponse
// @Router /task [post]
func CreateTaskHandler(store Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := uuid.New().String()
		store.SetTask(taskID, Task{Status: "in_progress"})

		go func() {
			time.Sleep(3 * time.Second)                                // эмуляция работы
			store.SetTask(taskID, Task{Status: "ready", Result: "42"}) // эмуляция результата
		}()

		json.NewEncoder(w).Encode(TaskResponse{TaskID: taskID})
	}
}

// @Summary Get task status
// @Produce json
// @Param taskID path string true "Task ID"
// @Success 200 {object} StatusResponse
// @Router /status/{taskID} [get]
func GetStatusHandler(store Storage) http.HandlerFunc {
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
// @Success 200 {string} string "result"
// @Failure 404 {string} string "not found"
// @Router /result/{taskID} [get]
func GetResultHandler(store Storage) http.HandlerFunc {
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
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		Session := Session{UserId: User.id,
			SessionId: SessionId}
		store.SetSession(Session)

		w.Header().Add("Authorization", "Bearer "+SessionId)
		w.WriteHeader(http.StatusOK)
	}
}
