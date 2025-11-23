// Package handlers предоставляет HTTP-обработчики для API сервиса назначения ревьюеров.
package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/2Empty/review-assigner/internal/models"
	"github.com/2Empty/review-assigner/internal/store"
)

// Handler предоставляет методы для обработки HTTP запросов.
type Handler struct {
	store *store.Store
}

// NewHandler создает новый экземпляр Handler с переданным store.
func NewHandler(store *store.Store) *Handler {
	return &Handler{store: store}
}

// ErrorResponse представляет структуру ответа с ошибкой.
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	resp := ErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = message
	writeJSON(w, status, resp)
}

func bindJSON(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("Failed to close request body: %v", err)
		}
	}()

	return json.Unmarshal(body, v)
}

// Health обрабатывает health check запросы.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CreateTeam создает новую команду с участниками.
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var team models.Team
	if err := bindJSON(r, &team); err != nil {
		writeError(w, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	if team.TeamName == "" || len(team.Members) == 0 {
		writeError(w, "INVALID_REQUEST", "team_name and members are required", http.StatusBadRequest)
		return
	}

	err := h.store.CreateTeam(r.Context(), team)
	if err != nil {
		if err == store.ErrTeamExists {
			writeError(w, "TEAM_EXISTS", "team_name already exists", http.StatusBadRequest)
			return
		}
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, team)
}

// GetTeam возвращает информацию о команде и её участниках.
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, "INVALID_REQUEST", "team_name is required", http.StatusBadRequest)
		return
	}

	team, err := h.store.GetTeam(r.Context(), teamName)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, "NOT_FOUND", "team not found", http.StatusNotFound)
			return
		}
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, team)
}

// SetUserActiveRequest представляет запрос на изменение активности пользователя.
type SetUserActiveRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

// SetUserActive устанавливает флаг активности пользователя.
func (h *Handler) SetUserActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SetUserActiveRequest
	if err := bindJSON(r, &req); err != nil {
		writeError(w, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.store.SetUserActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, "NOT_FOUND", "user not found", http.StatusNotFound)
			return
		}
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// UserReviewsResponse представляет ответ со списком PR пользователя.
type UserReviewsResponse struct {
	UserID       string             `json:"user_id"`
	PullRequests []PullRequestShort `json:"pull_requests"`
}

// PullRequestShort представляет сокращенную информацию о PR.
type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}

// GetUserReviews возвращает список PR, где пользователь назначен ревьювером.
func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, "INVALID_REQUEST", "user_id is required", http.StatusBadRequest)
		return
	}

	prs, err := h.store.GetUserReviews(r.Context(), userID)
	if err != nil {
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	prShorts := make([]PullRequestShort, len(prs))
	for i, pr := range prs {
		prShorts[i] = PullRequestShort{
			PullRequestID:   pr.PullRequestID,
			PullRequestName: pr.PullRequestName,
			AuthorID:        pr.AuthorID,
			Status:          pr.Status,
		}
	}

	writeJSON(w, http.StatusOK, UserReviewsResponse{
		UserID:       userID,
		PullRequests: prShorts,
	})
}

// CreatePRRequest представляет запрос на создание PR.
type CreatePRRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

// CreatePR создает новый PR и назначает до 2 ревьюверов из команды автора.
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreatePRRequest
	if err := bindJSON(r, &req); err != nil {
		writeError(w, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	pr, err := h.store.CreatePR(r.Context(), req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		switch err {
		case store.ErrPRExists:
			writeError(w, "PR_EXISTS", "PR id already exists", http.StatusConflict)
		case store.ErrNotFound:
			writeError(w, "NOT_FOUND", "author/team not found", http.StatusNotFound)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, pr)
}

// MergePRRequest представляет запрос на мерж PR.
type MergePRRequest struct {
	PullRequestID string `json:"pull_request_id"`
}

// MergePR помечает PR как MERGED (идемпотентная операция).
func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MergePRRequest
	if err := bindJSON(r, &req); err != nil {
		writeError(w, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	pr, err := h.store.MergePR(r.Context(), req.PullRequestID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, "NOT_FOUND", "PR not found", http.StatusNotFound)
			return
		}
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// ReassignReviewerRequest представляет запрос на переназначение ревьювера.
type ReassignReviewerRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}

// ReassignReviewerResponse представляет ответ на переназначение ревьювера.
type ReassignReviewerResponse struct {
	PR         *models.PullRequest `json:"pr"`
	ReplacedBy string              `json:"replaced_by"`
}

// ReassignReviewer переназначает ревьювера на другого участника из его команды.
func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReassignReviewerRequest
	if err := bindJSON(r, &req); err != nil {
		writeError(w, "INVALID_REQUEST", "invalid request body", http.StatusBadRequest)
		return
	}

	pr, newUserID, err := h.store.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			writeError(w, "NOT_FOUND", "PR or user not found", http.StatusNotFound)
		case store.ErrPRMerged:
			writeError(w, "PR_MERGED", "cannot reassign on merged PR", http.StatusConflict)
		case store.ErrNotAssigned:
			writeError(w, "NOT_ASSIGNED", "reviewer is not assigned to this PR", http.StatusConflict)
		case store.ErrNoCandidate:
			writeError(w, "NO_CANDIDATE", "no active replacement candidate in team", http.StatusConflict)
		default:
			writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, ReassignReviewerResponse{
		PR:         pr,
		ReplacedBy: newUserID,
	})
}

// GetStats возвращает статистику сервиса
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "METHOD_NOT_ALLOWED", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.store.GetStats(r.Context())
	if err != nil {
		writeError(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// SetupRoutes настраивает маршруты
func (h *Handler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)

	// Teams
	mux.HandleFunc("POST /team/add", h.CreateTeam)
	mux.HandleFunc("GET /team/get", h.GetTeam)

	// Users
	mux.HandleFunc("POST /users/setIsActive", h.SetUserActive)
	mux.HandleFunc("GET /users/getReview", h.GetUserReviews)

	// PullRequests
	mux.HandleFunc("POST /pullRequest/create", h.CreatePR)
	mux.HandleFunc("POST /pullRequest/merge", h.MergePR)
	mux.HandleFunc("POST /pullRequest/reassign", h.ReassignReviewer)
	// Stats
	mux.HandleFunc("GET /stats", h.GetStats)
}
