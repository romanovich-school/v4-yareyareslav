package auth

func JWTToLoginUserResponse(jwt JWT) LoginUserResponse {
	return LoginUserResponse{
		AccessToken:  jwt.AccessToken,
		RefreshToken: jwt.RefreshToken,
	}
}
