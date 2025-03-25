package main

import (
	"net/http"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"encoding/json"
	"internal/database"
	"internal/auth"
	"time"
	"database/sql"
)

// get all chirps
func getChirps(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	chirps, err := apiCfg.dbQueries.GetAllChirps(req.Context())
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error getting chirps: %v", err))
		return
	}
	output := []chirpParam{}
	for _, c := range chirps {
		output = append(output, chirpParam{
			ID: c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body: c.Body,
			UserID: c.UserID,
		})
	}
	respondWithJSON(wri, 200, output)
}

// get chirp by ID
func getChirpByID(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	chirpID, _ := uuid.Parse(req.PathValue("chirpID"))
	chirp, err := apiCfg.dbQueries.GetSingleChirp(req.Context(), chirpID)
	if err != nil {
		if strings.Contains(fmt.Sprint(err), "no rows in result set") {
			respondWithError(wri, 404, fmt.Sprint("Chirp not found"))
		} else {
			respondWithError(wri, 500, fmt.Sprintf("Error getting chirp: %v", err))
		}
		return
	}
	resBody := chirpParam {
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	}
	respondWithJSON(wri, 200, resBody)
}

// create a new chirp
func postChirp(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	type reqParam struct {
		Body string `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	
	// first decode the request
	decoder := json.NewDecoder(req.Body)
	reqBody := reqParam{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	if len(reqBody.Body) > 140 {
		respondWithError(wri, 400, "Chirp is too long")
		return
	}

	// make sure the user is valid
	bearer, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error getting bearer token: %v", err))
		return
	}
	user, err := auth.ValidateJWT(bearer, apiCfg.secret)
	if err != nil {
		respondWithError(wri, 401, "Unauthorized")
		return
	}
	
	chirp, err := apiCfg.dbQueries.CreateChirp(req.Context(), database.CreateChirpParams{Body: profanityFilter(reqBody.Body), UserID: user,})
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error creating chirp: %v", err))
		return
	}
	resBody := chirpParam{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.CreatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	}
	respondWithJSON(wri, 201, resBody)
}

// create a new user
func postUser(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	type reqParam struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}
	
	// first decode the request
	decoder := json.NewDecoder(req.Body)
	reqBody := reqParam{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error decoding request: %v", err))
		return
	}

	hashword, err := auth.HashPassword(reqBody.Password)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error hashing password: %v", err))
	}
	user, err := apiCfg.dbQueries.CreateUser(req.Context(), database.CreateUserParams{Email: reqBody.Email, HashedPassword: hashword})
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error creating user: %v", err))
		return
	}
	resBody := userParam{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.CreatedAt,
		Email: user.Email,
	}
	respondWithJSON(wri, 201, resBody)
}

// login
func postLogin(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	type reqParam struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	// first decode the request
	decoder := json.NewDecoder(req.Body)
	reqBody := reqParam{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error decoding request: %v", err))
		return
	}

	// check authorization
	user, err := apiCfg.dbQueries.GetUserByEmail(req.Context(), reqBody.Email)
	if err != nil {
		respondWithError(wri, 401, "Incorrect username or password")
	}
	err = auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
	if err != nil {
		respondWithError(wri, 401, "Incorrect username or password")
	}

	// get jwt token
	dura, _ := time.ParseDuration(fmt.Sprintf("3600s"))
	jwtToken, err := auth.MakeJWT(user.ID, apiCfg.secret, dura)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error getting JWT token: %v", err))
	}
	tokenStr := auth.MakeRefreshToken()
	refDura, _ := time.ParseDuration(fmt.Sprintf("1440h"))
	refreshToken, err := apiCfg.dbQueries.CreateToken(req.Context(), database.CreateTokenParams{
		Token: tokenStr,
		UserID: user.ID,
		ExpiresAt: sql.NullTime{Time: time.Now().Add(refDura), Valid: true},
	})
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error getting refresh token: %v", err))
	}
	
	resBody := userParam{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.CreatedAt,
		Email: user.Email,
		Token: jwtToken,
		RefreshToken: refreshToken.Token,
	}
	respondWithJSON(wri, 200, resBody)
}

// get refreshed jwt token
func refresh(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	// `the json:"token"` bit is essential, yes even more essential than that x_x
	type resParam struct {
		Token string `json:"token"` 
	}
	bearer, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error getting bearer token: %v", err))
		return
	}
	userWithExpiration, err := apiCfg.dbQueries.GetUserFromRefreshToken(req.Context(), bearer)
	if err != nil {
		respondWithError(wri, 401, fmt.Sprintf("Unauthorized: %v", err))
		return
	}
	if userWithExpiration.RevokedAt.Valid == true {
		respondWithError(wri, 401, "Revoked")
		return
	}
	now := time.Now()
	if userWithExpiration.ExpiresAt.Time.Before(now) {
		respondWithError(wri, 401, "Expired")
		return
	}
	
	dura, _ := time.ParseDuration(fmt.Sprintf("3600s"))
	jwtToken, err := auth.MakeJWT(userWithExpiration.UserID, apiCfg.secret, dura)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error getting JWT token: %v", err))
	}
	resBody := resParam{
		Token: jwtToken,
	}
	respondWithJSON(wri, 200, resBody)
}

// revoke the refresh token
func revoke(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	bearer, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error getting bearer token: %v", err))
		return
	}
	err = apiCfg.dbQueries.RevokeToken(req.Context(), bearer)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error revoking token: %v", err))
		return
	}
	wri.WriteHeader(204)
}

// change user's email and password
func putUser(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	type reqParam struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	// validate the user
	bearer, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(wri, 401, fmt.Sprintf("Error getting bearer token: %v", err))
		return
	}
	user, err := auth.ValidateJWT(bearer, apiCfg.secret)
	if err != nil {
		respondWithError(wri, 401, "Unauthorized")
		return
	}

	// decode the request
	decoder := json.NewDecoder(req.Body)
	reqBody := reqParam{}
	err = decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error decoding request: %v", err))
		return
	}

	hashword, err := auth.HashPassword(reqBody.Password)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error hashing password: %v", err))
	}
	updatedUser, err := apiCfg.dbQueries.UpdateEmailAndPassword(req.Context(), database.UpdateEmailAndPasswordParams{
		ID: user,
		Email: reqBody.Email,
		HashedPassword: hashword,
	})
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error updating user: %v", err))
	}

	resBody := userParam{
		ID: updatedUser.ID,
		CreatedAt: updatedUser.CreatedAt,
		UpdatedAt: updatedUser.CreatedAt,
		Email: updatedUser.Email,
		//Token: updatedUser.Token,
		//RefreshToken: updatedUser.RefreshToken,
	}
	respondWithJSON(wri, 200, resBody)
}

func deleteChirp(wri http.ResponseWriter, req *http.Request, apiCfg apiConfig) {
	// validate the user
	bearer, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(wri, 401, fmt.Sprintf("Error getting bearer token: %v", err))
		return
	}
	user, err := auth.ValidateJWT(bearer, apiCfg.secret)
	if err != nil {
		respondWithError(wri, 403, "JWT is invalid")
		return
	}

	// get chirp
	chirpID, _ := uuid.Parse(req.PathValue("chirpID"))
	chirp, err := apiCfg.dbQueries.GetSingleChirp(req.Context(), chirpID)
	if err != nil {
		if strings.Contains(fmt.Sprint(err), "no rows in result set") {
			respondWithError(wri, 404, fmt.Sprint("Chirp not found"))
		} else {
			respondWithError(wri, 404, fmt.Sprintf("Error getting chirp: %v", err))
		}
		return
	}

	// compare chirp owner to user
	if chirp.UserID != user {
		respondWithError(wri, 403, "Unauthorized")
		return
	}

	// delete the chrip
	err = apiCfg.dbQueries.DeleteSingleChirp(req.Context(), chirp.ID)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error deleting chirp: %v", err))
	}
	wri.WriteHeader(204)
}