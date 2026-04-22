package captcha

// noopCaptchaService is the interface for noop captcha service
type noopCaptchaService struct {
}

func (s *noopCaptchaService) Generate() (any, error) {
	return nil, nil
}

func (s *noopCaptchaService) Verify(captcha string) error {
	return nil
}
