package service

import (
	"github.com/Nina-99/yottu-api/data/request"
)

type AuthAccountService interface {
	Register(req request.RegisterRequest) (string, error)
	Login(req request.LoginRequest) (string, error)
	SocialLogin(req request.SocialLoginRequest) (string, error)
}
