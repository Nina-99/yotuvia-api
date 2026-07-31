package service

import "github.com/Nina-99/yottu-api/data/request"

type AuthResult struct {
	Token     string
	UserID    uint
	Email     string
	FirstName string
	LastName  string
	Role      string
}

type AuthAccountService interface {
	Register(req request.RegisterRequest) (AuthResult, error)
	Login(req request.LoginRequest) (AuthResult, error)
	SocialLogin(req request.SocialLoginRequest) (AuthResult, error)
	RegisterPersonnel(req request.RegisterPersonnelRequest) error
	CheckEmail(req request.CheckEmailRequest) (bool, error)
}
