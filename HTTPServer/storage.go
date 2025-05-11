package main

import (
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type Task struct {
	Status string
	Result string
}

type Session struct {
	UserId    string
	SessionId string
}

type User struct {
	id    string
	login string
	hash  string
}

type Storage interface {
	SetTask(id string, task Task)
	GetTask(id string) (Task, bool)
	RegisterUser(id string, username string, password string) error
	GetUserByLogin(login string) (User, bool)
	SetSession(session Session)
	GetSession(SessionId string) (Session, bool)
}

type InMemoryStorage struct {
	mu       sync.RWMutex
	tasks    map[string]Task
	users    map[string]User
	sessions map[string]Session
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		tasks:    make(map[string]Task),
		users:    make(map[string]User),
		sessions: make(map[string]Session),
	}
}

func (s *InMemoryStorage) SetTask(id string, task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[id] = task
}

func (s *InMemoryStorage) GetTask(id string) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok
}

func (s *InMemoryStorage) RegisterUser(id string, username string, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; exists {
		return fmt.Errorf("user alredy exists")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("user alredy exists")
	}
	s.users[username] = User{id: id, login: username, hash: string(hashedPassword)}
	return nil
}

func (s *InMemoryStorage) GetUserByLogin(login string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[login]
	return user, exists
}

func (s *InMemoryStorage) SetSession(session Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.SessionId] = session
}

func (s *InMemoryStorage) GetSession(SessionId string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, exists := s.sessions[SessionId]
	return session, exists
}
