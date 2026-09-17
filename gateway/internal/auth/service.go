package auth

import (
	"context"
)


type AuthService struct {}

func NewAuthService() *AuthService {
	s := &AuthService{}
	return s
}

func (s *AuthService) Register(ctx context.Context, req RegisterUserRequest) (RegisterUserResponse, error) {
	return RegisterUserResponse{}, nil
}

func (s *AuthService) Login(ctx context.Context, req LoginUserRequest) (LoginUserResponse, error) {

	

	return LoginUserResponse{}, nil
}

func (s *AuthService) Refresh(ctx context.Context, req RefreshTokenRequest) (RefreshTokenResponse, error) {
	return RefreshTokenResponse{}, nil
}