package domain

import "time"

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    time.Duration
}

type Identity struct {
	UserID int64
	Role   string
}
