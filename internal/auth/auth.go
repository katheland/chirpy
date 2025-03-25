package auth

import (
	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
	"fmt"
	"net/http"
	"strings"
	"crypto/rand"
	"encoding/hex"
	"strconv"
)

// turn a password into a hash
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(hash), err
}

// check a password and a hash
func CheckPasswordHash(password, hash string) (error) {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// get a jwt token
func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject: userID.String(),
	})
	return token.SignedString([]byte(tokenSecret))
}

// validate a jwt token
func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("Error in ParseWithClaims: %v", err)
	}
	idString, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("Error in GetSubject: %v", err)
	}
	idUU, err := uuid.Parse(idString)
	return idUU, err
}

// get a bearer token from the header
func GetBearerToken(headers http.Header) (string, error) {
	token, ok := headers["Authorization"]
	if ok == false {
		return "", fmt.Errorf("Error getting the token from the header")
	}
	return strings.TrimSpace(strings.TrimPrefix(token[0], "Bearer")), nil
}

// get a refresh token
func MakeRefreshToken() (string, error) {
	randNum, err := rand.Read(make([]byte, 32))
	return hex.EncodeToString([]byte(strconv.Itoa(randNum))), err
}