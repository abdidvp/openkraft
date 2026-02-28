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

// routeRequest dispatches requests based on method and path combinations.
// This intentionally violates cognitive complexity limits for test fixture purposes.
func (h *UserHandler) routeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if r.URL.Path == "/users" {
			if r.URL.Query().Get("search") != "" {
				if r.URL.Query().Get("page") != "" {
					for i := 0; i < 10; i++ {
						if i > 5 {
							w.Write([]byte("paginated search"))
						}
					}
				}
			} else if r.URL.Query().Get("sort") != "" {
				w.Write([]byte("sorted list"))
			}
		} else if r.URL.Path == "/users/export" {
			if r.Header.Get("Accept") == "text/csv" {
				w.Write([]byte("csv"))
			} else if r.Header.Get("Accept") == "application/pdf" {
				w.Write([]byte("pdf"))
			} else {
				w.Write([]byte("json"))
			}
		}
	} else if r.Method == "POST" {
		if r.URL.Path == "/users" {
			h.HandleCreateUser(w, r)
		} else if r.URL.Path == "/users/bulk" {
			for i := 0; i < 100; i++ {
				if i%10 == 0 {
					w.Write([]byte("progress"))
				}
			}
		}
	} else if r.Method == "DELETE" {
		if r.URL.Query().Get("force") == "true" {
			w.Write([]byte("force deleted"))
		} else {
			w.Write([]byte("soft deleted"))
		}
	}
}
