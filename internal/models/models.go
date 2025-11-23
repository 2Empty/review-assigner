// Package models содержит структуры данных для сервиса назначения ревьюеров.
package models

import "time"

// User представляет пользователя системы.
type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

// TeamMember представляет участника команды.
type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// Team представляет команду пользователей.
type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

// PullRequest представляет pull request.
type PullRequest struct {
	PullRequestID     string     `json:"pull_request_id"`
	PullRequestName   string     `json:"pull_request_name"`
	AuthorID          string     `json:"author_id"`
	Status            string     `json:"status"` //[OPEN, MERGED]
	AssignedReviewers []string   `json:"assigned_reviewers"`
	CreatedAt         time.Time  `json:"createdAt"`
	MergedAt          *time.Time `json:"mergedAt"`
}

// PullRequestShort представляет сокращенную информацию о PR.
type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"` //[OPEN, MERGED]
}

// Stats представляет общую статистику сервиса
type Stats struct {
	TotalPRs      int            `json:"total_prs"`
	PRsByStatus   map[string]int `json:"prs_by_status"`
	ReviewsByUser map[string]int `json:"reviews_by_user"`
	ActiveUsers   int            `json:"active_users"`
	TotalTeams    int            `json:"total_teams"`
}
