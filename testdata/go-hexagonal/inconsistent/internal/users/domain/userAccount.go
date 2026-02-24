package domain

import (
	"errors"
	"time"
)

// UserAccount represents a user in the system.
type UserAccount struct {
	ID        string
	firstName string
	lastName  string
	email     string
	createdAt time.Time
}

// NewUserAccount creates a new UserAccount.
func NewUserAccount(firstName string, lastName string, email string) *UserAccount {
	return &UserAccount{
		firstName: firstName,
		lastName:  lastName,
		email:     email,
		createdAt: time.Now(),
	}
}

// FirstName returns the user's first name.
func (u *UserAccount) FirstName() string { return u.firstName }

// LastName returns the user's last name.
func (u *UserAccount) LastName() string { return u.lastName }

// Email returns the user's email.
func (u *UserAccount) Email() string { return u.email }

// FullName returns the user's full name.
func (u *UserAccount) FullName() string {
	return u.firstName + " " + u.lastName
}

// Validate checks that the user account has valid data.
func (u *UserAccount) Validate() error {
	if u.firstName == "" {
		return errors.New("first name is required")
	}
	if u.email == "" {
		return errors.New("email is required")
	}
	return nil
}
