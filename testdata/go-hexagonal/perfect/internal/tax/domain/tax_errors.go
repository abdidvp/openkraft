package domain

import "errors"

var (
	ErrTaxRuleNotFound = errors.New("tax rule not found")
	ErrTaxRuleInvalid  = errors.New("tax rule is invalid")
	ErrDuplicateRule   = errors.New("duplicate tax rule")
)
