package auth_utils

import (
	"time"

	"github.com/dgrijalva/jwt-go"

	"github.com/soulonmysleevethroughapinhole/instinct_webrtc/utils/rest_errors"
)

//alright what do I need

//isexpired
//validate it's real shit
//then just get username into a struct
//otherwise start bitching and throw whimsical errors

// ALL OF THIS WILL HAVE TO BE MOVED TO THE CORE, THIS IS IN NEARLY EVERY SERVICE BUT I FUCKED GOMODULES UP
var jwtKey = []byte("my_secret_key")

type Claims struct {
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expiresat"`
	jwt.StandardClaims
}

func Authenticate(authHeader string) (string, rest_errors.RestErr) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(authHeader, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			return "", rest_errors.NewBadRequestError("auth failed")
		}
		return "", rest_errors.NewBadRequestError("auth failed")
	}
	if !token.Valid {
		return "", rest_errors.NewBadRequestError("auth failed")
	}
	if time.Unix(claims.ExpiresAt, 0).Before(time.Now().UTC()) {
		//aight so I only extended claims here not sure why it works or how it is generatied in the first place
		return "", rest_errors.NewBadRequestError("token expired")
	}

	if claims.Username == "" {
		return "", rest_errors.NewBadRequestError("username not defined, if you see this, pack your bags, this is the best possible time to book your one-way ticket to your chosen non-extradition country")
	}
	return claims.Username, nil
}
