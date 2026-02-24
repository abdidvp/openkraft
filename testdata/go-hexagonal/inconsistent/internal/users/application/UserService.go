package application

import "example.com/inconsistent/internal/users/domain"

// UserService handles user business logic.
type UserService struct{}

// NewUserService creates a new UserService.
func NewUserService() *UserService {
	return &UserService{}
}

// CreateUser creates and validates a new user account.
func (s *UserService) CreateUser(firstName string, lastName string, email string) (*domain.UserAccount, error) {
	user := domain.NewUserAccount(firstName, lastName, email)
	if err := user.Validate(); err != nil {
		return nil, err
	}
	return user, nil
}
