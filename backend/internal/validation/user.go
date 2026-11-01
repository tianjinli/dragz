// Package validation holds the field rules shared by the login and account
// handlers.
package validation

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidEmail means the address does not look like an email.
	ErrInvalidEmail = errors.New("invalid email")
	// ErrInvalidMobile means the number is not a mainland mobile number.
	ErrInvalidMobile = errors.New("invalid mobile")
	// ErrInvalidPassword means the password failed the length, complexity or run check.
	ErrInvalidPassword = errors.New("invalid password")
	// ErrInvalidUsername means the username does not match the allowed shape.
	ErrInvalidUsername = errors.New("invalid username")
)

var (
	mainlandMobileRegexp = regexp.MustCompile(`^1[3-9]\d{9}$`)
	usernameRegexp       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{3,63}$`)
)

// ValidateUsername enforces a leading letter, then letters, digits or underscores, 4 to 64 characters in total.
func ValidateUsername(username string) error {
	if usernameRegexp.MatchString(username) {
		return nil
	}
	return ErrInvalidUsername
}

// ValidatePassword enforces 8–64 characters, the bcrypt byte limit, at least
// three of four ASCII character classes, and sequential/repeated run checks.
func ValidatePassword(password, username string, minLength int) error {
	if minLength < 8 || minLength > 64 || utf8.RuneCountInString(password) < minLength || utf8.RuneCountInString(password) > 64 || len(password) > 72 {
		return ErrInvalidPassword
	}
	if hasSequentialOrRepeatedRun(password, 6) {
		return ErrInvalidPassword
	}
	classes := 0
	var upper, lower, digit, special bool
	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			upper = true
		case char >= 'a' && char <= 'z':
			lower = true
		case char >= '0' && char <= '9':
			digit = true
		case (char >= '!' && char <= '/') || (char >= ':' && char <= '@') || (char >= '[' && char <= '`') || (char >= '{' && char <= '~'):
			special = true
		}
	}
	for _, present := range []bool{upper, lower, digit, special} {
		if present {
			classes++
		}
	}
	if classes < 3 {
		return ErrInvalidPassword
	}
	return nil
}

func hasSequentialOrRepeatedRun(value string, minimum int) bool {
	chars := []rune(strings.ToLower(value))
	if len(chars) < minimum {
		return false
	}
	for start := 0; start+minimum <= len(chars); start++ {
		ascending, descending, repeated := true, true, true
		for index := start + 1; index < start+minimum; index++ {
			ascending = ascending && chars[index] == chars[index-1]+1
			descending = descending && chars[index] == chars[index-1]-1
			repeated = repeated && chars[index] == chars[index-1]
		}
		if ascending || descending || repeated {
			return true
		}
	}
	return false
}

// ValidateMobile accepts an empty value (the field is optional) or a mainland mobile number.
func ValidateMobile(mobile string) error {
	if mobile == "" || mainlandMobileRegexp.MatchString(mobile) {
		return nil
	}
	return ErrInvalidMobile
}

// ValidateEmail accepts an empty value (the field is optional) or a single-@ address with a dotted domain.
func ValidateEmail(email string) error {
	if email == "" {
		return nil
	}
	if len(email) > 255 {
		return ErrInvalidEmail
	}
	if strings.ContainsAny(email, " \t\r\n<>()[],:;\\\"") {
		return ErrInvalidEmail
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at != strings.IndexByte(email, '@') || at == len(email)-1 {
		return ErrInvalidEmail
	}
	domain := email[at+1:]
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || !strings.Contains(domain, ".") {
		return ErrInvalidEmail
	}
	return nil
}
