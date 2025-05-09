package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/nickabs/signals"
	"github.com/nickabs/signals/internal/auth"
	"github.com/nickabs/signals/internal/database"
	"github.com/nickabs/signals/internal/helpers"
)

type UserHandler struct {
	cfg *signals.ServiceConfig
}

func NewUserHandler(cfg *signals.ServiceConfig) *UserHandler {
	return &UserHandler{cfg: cfg}
}

// CreateUserHandler godoc
//
//	@Summary	Create user
//	@Tags		auth
//
//	@Param		request	body		handlers.CreateUserHandler.createUserRequest	true	"user details"
//
//	@Success	201		{object}	handlers.CreateUserHandler.createUserResponse
//	@Failure	400		{object}	signals.ErrorResponse
//	@Failure	409		{object}	signals.ErrorResponse
//	@Failure	500		{object}	signals.ErrorResponse
//
//	@Router		/api/users [post]
func (u *UserHandler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	type createUserRequest struct {
		Password string `json:"password" example:"password"`
		Email    string `json:"email" example:"example@example.com"`
	}

	var req createUserRequest

	type createUserResponse struct {
		ID uuid.UUID `json:"id" example:"68fb5f5b-e3f5-4a96-8d35-cd2203a06f73"`
	}

	var newUser = database.User{}

	var res = createUserResponse{}

	authService := auth.NewAuthService(u.cfg)

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeMalformedBody, fmt.Sprintf("could not decode request body: %v", err))
		return
	}

	if req.Email == "" || req.Password == "" {
		helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeMalformedBody, "you must supply {email} and {password}")
		return
	}

	exists, err := u.cfg.DB.ExistsUserWithEmail(r.Context(), req.Email)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeDatabaseError, fmt.Sprintf("database error: %v", err))
		return
	}
	if exists {
		helpers.RespondWithError(w, r, http.StatusConflict, signals.ErrCodeUserAlreadyExists, "a user already exists this email address")
		return
	}

	hashedPassword, err := authService.HashPassword(req.Password)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeInternalError, fmt.Sprintf("could not hash password: %v", err))
		return
	}

	newUser, err = u.cfg.DB.CreateUser(r.Context(), database.CreateUserParams{
		HashedPassword: hashedPassword,
		Email:          req.Email,
	})
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeDatabaseError, fmt.Sprintf("could not create user: %v", err))
		return
	}

	res = createUserResponse{
		ID: newUser.ID,
	}
	helpers.RespondWithJSON(w, http.StatusCreated, res)
}

// UpdateUserHandler godoc
//
//	@Summary		Update user
//	@Description	update email and/or password
//	@Tags			auth
//
//	@Param			request	body		handlers.UpdateUserHandler.updateUserRequest	true	"user details"
//
//	@Success		200		{object}	handlers.UpdateUserHandler.updateUserResponse
//	@Failure		400		{object}	signals.ErrorResponse
//	@Failure		401		{object}	signals.ErrorResponse
//	@Failure		404		{object}	signals.ErrorResponse
//	@Failure		500		{object}	signals.ErrorResponse
//
//	@Security		BearerAccessToken
//
//	@Router			/api/users [put]
func (u *UserHandler) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	type updateUserRequest struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	type updateUserResponse struct {
		Email string `json:"email"`
	}

	authService := auth.NewAuthService(u.cfg)

	ctx := r.Context()

	userID, ok := ctx.Value(signals.UserIDKey).(uuid.UUID)
	if !ok {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeInternalError, "did not receive userID from middleware")
	}

	defer r.Body.Close()

	req := updateUserRequest{}

	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeMalformedBody, fmt.Sprintf("could not decode request body: %v", err))
		return
	}

	if req.Email == "" || req.Password == "" {
		helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeMalformedBody, "expecting a email or password in http body")
		return
	}

	currentUser, err := u.cfg.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		helpers.RespondWithError(w, r, http.StatusUnauthorized, signals.ErrCodeResourceNotFound, fmt.Sprintf("could not find a user with the token UUID: %v", userID))
		return
	}

	// prepare update params
	updateParams := database.UpdateUserEmailAndPasswordParams{
		ID:             currentUser.ID,
		Email:          currentUser.Email,
		HashedPassword: currentUser.HashedPassword,
	}

	if req.Email != "" {
		updateParams.Email = req.Email
	}

	if req.Password != "" {
		updateParams.HashedPassword, err = authService.HashPassword(req.Password)
		if err != nil {
			helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeInternalError, fmt.Sprintf("server error: %v", err))
			return
		}
	}
	rowsAffected, err := u.cfg.DB.UpdateUserEmailAndPassword(r.Context(), updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.RespondWithError(w, r, http.StatusBadRequest, signals.ErrCodeUserNotFound, "user not found")
			return
		}
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeDatabaseError, fmt.Sprintf("database error: %v", err))
		return
	}
	if rowsAffected != 1 {
		helpers.RespondWithError(w, r, http.StatusInternalServerError, signals.ErrCodeDatabaseError, "error updating user")
		return
	}

	helpers.RespondWithJSON(w, http.StatusOK, updateUserResponse{
		Email: updateParams.Email,
	})

}
