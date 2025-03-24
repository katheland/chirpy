package auth

import (
	"testing"
	"github.com/google/uuid"
	"time"
)

func TestHash(t *testing.T) {
	cases := []struct{
		input string
	}{
		{
			input: "",
		},
		{
			input: "123456",
		},
		{
			input: "abcdef",
		},
	}
	for _, c := range cases {
		hash, err := HashPassword(c.input)
		if err != nil {
			t.Errorf("Input: %v\nError with HashPassword %v\nOutput: %v", c.input, err, hash)
			continue
		}
		err = CheckPasswordHash(c.input, hash)
		if err != nil {
			t.Errorf("Input: %v\nError with CheckPasswordHash %v\nHash: %v", c.input, err, hash)
		}
	}
}

func TestJWT(t *testing.T) {
	cases := []struct{
		userID uuid.UUID
		tokenSecret string
		expiresIn string
	}{
		{
			userID: uuid.New(),
			tokenSecret: "test",
			expiresIn: "10s",
		},
	}
	fails := []struct{
		userID uuid.UUID
		tokenSecret string
		expiresIn string
		checkSecret string
	}{
		{
			userID: uuid.New(),
			tokenSecret: "test",
			expiresIn: "-10s",
			checkSecret: "test",
		},
		{
			userID: uuid.New(),
			tokenSecret: "test",
			expiresIn: "10s",
			checkSecret: "notTest",
		},
	}
	for _, c := range cases {
		expireDuration, _ := time.ParseDuration(c.expiresIn)
		jwt, err := MakeJWT(c.userID, c.tokenSecret, expireDuration)
		if err != nil {
			t.Errorf("userID: %v\ntokenSecret: %v\nexpiresIn: %v\nError: %v", c.userID, c.tokenSecret, c.expiresIn, err)
			continue
		}
		user, err := ValidateJWT(jwt, c.tokenSecret)
		if err != nil {
			t.Errorf("jwt: %v\ntokenSecret: %v\nError: %v", jwt, c.tokenSecret, err)
			continue
		}
		if user != c.userID {
			t.Errorf("user: %v\nuserID: %v\nThey don't match.", user, c.userID)
		}
	}
	for _, f := range fails {
		expireDuration, _ := time.ParseDuration(f.expiresIn)
		jwt, err := MakeJWT(f.userID, f.tokenSecret, expireDuration)
		if err != nil {
			continue
		}
		user, err := ValidateJWT(jwt, f.checkSecret)
		if err != nil {
			continue
		}
		if user != f.userID {
			continue
		}
		t.Errorf("userID: %v\ntokenSecret: %v\nexpireDuration: %v checkSecret: %v\njwt: %v\nuser: %v\n This should have failed.", f.userID, f.tokenSecret, f.expiresIn, f.checkSecret, jwt, user)
	}
}