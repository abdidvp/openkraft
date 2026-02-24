package domain

import "testing"

func Test_new_user_account(t *testing.T) {
	user := NewUserAccount("John", "Doe", "john@example.com")

	if user.FirstName() != "John" {
		t.Errorf("expected first name John, got %s", user.FirstName())
	}
	if user.LastName() != "Doe" {
		t.Errorf("expected last name Doe, got %s", user.LastName())
	}
	if user.Email() != "john@example.com" {
		t.Errorf("expected email john@example.com, got %s", user.Email())
	}
}

func Test_user_account_validate(t *testing.T) {
	valid_user := NewUserAccount("Jane", "Doe", "jane@example.com")
	if err := valid_user.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	missing_name := NewUserAccount("", "Doe", "test@example.com")
	if err := missing_name.Validate(); err == nil {
		t.Error("expected error for missing first name")
	}

	missing_email := NewUserAccount("Jane", "Doe", "")
	if err := missing_email.Validate(); err == nil {
		t.Error("expected error for missing email")
	}
}

func Test_user_full_name(t *testing.T) {
	user := NewUserAccount("John", "Doe", "john@example.com")
	expected_name := "John Doe"
	if user.FullName() != expected_name {
		t.Errorf("expected full name %s, got %s", expected_name, user.FullName())
	}
}
