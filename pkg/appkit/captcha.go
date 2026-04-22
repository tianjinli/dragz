package appkit

// https://github.com/stretchr/testify

// CaptchaService is the interface for captcha service
type CaptchaService interface {
	Generate() (any, error)
	Verify(captcha string) error
}
