package validation

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		valid    bool
	}{
		{name: "too short", password: "Aa1!abc", valid: false},
		{name: "two classes", password: "abcdefgh1", valid: false},
		{name: "three classes", password: "Az1xY2qw", valid: true},
		{name: "with special", password: "Ax9!By2@", valid: true},
		{name: "sequential characters", password: "Aa1!abcdef", valid: false},
		{name: "repeated characters", password: "Aa1!111111", valid: false},
		{name: "too long", password: strings.Repeat("A", 61) + "a1!", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidatePassword(test.password, "example", 8) == nil; got != test.valid {
				t.Fatalf("ValidatePassword(%q) valid=%v, want %v", test.password, got, test.valid)
			}
		})
	}
}

func TestValidatePasswordAllowsUsername(t *testing.T) {
	if ValidatePassword("Example1!x", "example", 8) != nil {
		t.Fatal("password containing the username should be allowed")
	}
}

func TestValidateUsername(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{"admin1", true},
		{"user_name", true},
		{"1admin", false},
		{"ab", false},
		{"admin-name", false},
		{"admin.name", false},
		{"管理员", false},
		{"admin 1", false},
	} {
		if got := ValidateUsername(test.value) == nil; got != test.valid {
			t.Fatalf("ValidateUsername(%q) valid=%v, want %v", test.value, got, test.valid)
		}
	}
}

func TestValidateMobile(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{{"", true}, {"13800138000", true}, {"12800138000", false}, {"1380013800", false}, {"+8613800138000", false}} {
		if got := ValidateMobile(test.value) == nil; got != test.valid {
			t.Fatalf("ValidateMobile(%q) valid=%v, want %v", test.value, got, test.valid)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{{"", true}, {"admin@example.com", true}, {"admin@example", false}, {"Admin <admin@example.com>", false}, {"admin @example.com", false}} {
		if got := ValidateEmail(test.value) == nil; got != test.valid {
			t.Fatalf("ValidateEmail(%q) valid=%v, want %v", test.value, got, test.valid)
		}
	}
}
