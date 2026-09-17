package auth

func RegisterUserRequestToUser(req RegisterUserRequest) User {
	return User{
		ID:       nil,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	}
}

func UserToRegisterUserResponse(user User, accessToken, refreshToken string) RegisterUserResponse {
	return RegisterUserResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: RegisterUserReponseUser{
			Email: user.Email,
			Password: user.Password,
			Role: user.Role,
		},
	}
}

func UserToLoginUserResponse(user User, accessToken, refreshToken string) LoginUserResponse {
	return LoginUserResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}