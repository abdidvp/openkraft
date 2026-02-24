package http

import (
	"encoding/json"
	"net/http"

	"example.com/inconsistent/internal/users/application"
)

// UserHandler handles HTTP requests for users.
type UserHandler struct {
	service *application.UserService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(service *application.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// CreateUserRequest represents the request body for creating a user.
type CreateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// HandleCreateUser handles POST requests to create a user.
func (h *UserHandler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.service.CreateUser(req.FirstName, req.LastName, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"name":  user.FullName(),
		"email": user.Email(),
	})
}
