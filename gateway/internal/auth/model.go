package auth

type UserID int

type User struct {
	ID       *UserID
	Email    string
	Password string
	Role     string
}

type JWT struct {
	AccessToken  string
	RefreshToken string
}