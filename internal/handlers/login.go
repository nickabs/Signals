package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nickabs/signals"
	"github.com/nickabs/signals/internal/auth"
	"github.com/nickabs/signals/internal/database"
	"github.com/nickabs/signals/internal/helpers"
)

type LoginHandler struct {
	cfg *signals.ServiceConfig
}

func NewLoginHandler(cfg *signals.ServiceConfig) *LoginHandler {
	return &LoginHandler{cfg: cfg}
}

// LoginHandler godoc
//
//	@Summary		Login
//	@Description	The response body includes an access token and a refresh_token.
//	@Description	The access_token is valid for 1 hour.
//	@Description
//	@Description	Use the refresh_token with the /api/refresh endpoint to renew the access_token.
//	@Description	The refresh_token lasts 60 days unless it is revoked earlier.
//	@Description	To renew the refresh_token, log in again.
//	@Tags			auth
//
//	@Param			request	body		handlers.LoginHandler.loginRequest	true	"user details"
//
//	@Success		200		{object}	handlers.LoginHandler.loginResponse
//	@Failure		400		{object}	signals.ErrorResponse
//	@Failure		401		{object}	signals.ErrorResponse
//	@Failure		500		{object}	signals.ErrorResponse
//
//	@Router			/api/login [post]
func (l *LoginHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	type loginRequest struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	type loginResponse struct {
		UserID       uuid.UUID `json:"user_id" example:"68fb5f5b-e3f5-4a96-8d35-cd2203a06f73"`
		CreatedAt    time.Time `json:"created_at" example:"2025-05-09T05:41:22.57328+01:00"`
		AccessToken  string    `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJTaWduYWxTZXJ2ZXIiLCJzdWIiOiI2OGZiNWY1Yi1lM2Y1LTRhOTYtOGQzNS1jZDIyMDNhMDZmNzMiLCJleHAiOjE3NDY3NzA2MzQsImlhdCI6MTc0Njc2NzAzNH0.3OdnUNgrvt1Zxs9AlLeaC9DVT6Xwc6uGvFQHb6nDfZs"`
		RefreshToken string    `json:"refresh_token" example:"fb948e0b74de1f65e801b4e70fc9c047424ab775f2b4dc5226f472f3b6460c37"`
	}

	var req loginRequest

	authService := auth.NewAuthService(l.cfg)

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeMalformedBody, fmt.Sprintf("could not decode request body: %v", err))
		return
	}

	exists, err := l.cfg.DB.ExistsUserWithEmail(r.Context(), req.Email)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeDatabaseError, fmt.Sprintf("database error: %v", err))
		return
	}
	if !exists {
		helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeUserNotFound, "no user found with this email address")
		return
	}

	user, err := l.cfg.DB.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeDatabaseError, fmt.Sprintf("database error: %v", err))
		return
	}

	err = authService.CheckPasswordHash(user.HashedPassword, req.Password)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusUnauthorized, signals.ErrCodeAuthenticationFailure, "Incorrect email or password")
		return
	}

	accessToken, err := authService.GenerateAccessToken(user.ID, l.cfg.SecretKey, signals.AccessTokenExpiry)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeTokenError, fmt.Sprintf("error creating access token: %v", err))
		return
	}

	refreshToken, err := authService.GenerateRefreshToken()
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeTokenError, fmt.Sprintf("error creating refresh token: %v", err))
		return
	}

	_, err = l.cfg.DB.InsertRefreshToken(r.Context(), database.InsertRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(signals.RefreshTokenExpiry),
	})
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeDatabaseError, fmt.Sprintf("could not create user: %v", err))
		return
	}

	res := loginResponse{
		UserID:       user.ID,
		CreatedAt:    user.CreatedAt,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	helpers.RespondWithJSON(w, http.StatusOK, res)
}
