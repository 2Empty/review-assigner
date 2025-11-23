// Package store предоставляет слой доступа к данным для сервиса назначения ревьюеров.
package store

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"github.com/2Empty/review-assigner/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store предоставляет методы для работы с данными.
type Store struct {
	Pool *pgxpool.Pool
}

// New создает новый экземпляр Store.
func New(ctx context.Context) (*Store, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Store{Pool: pool}, nil
}

// Close закрывает соединение с базой данных
func (s *Store) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}

// CreateTeam создает новую команду в базе данных.
func (s *Store) CreateTeam(ctx context.Context, t models.Team) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			log.Printf("Failed to rollback transaction: %v", err)
		}
	}()

	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", t.TeamName)
	if err != nil {
		return fmt.Errorf("lock table: %w", err)
	}

	var existingTeamCount int
	err = tx.QueryRow(ctx, `
        SELECT COUNT(*) FROM users WHERE team_name = $1`,
		t.TeamName).Scan(&existingTeamCount)

	if err != nil {
		return fmt.Errorf("check team existence: %w", err)
	}

	if existingTeamCount > 0 {
		return ErrTeamExists
	}

	for _, m := range t.Members {
		_, err = tx.Exec(ctx, `
            INSERT INTO users (user_id, username, team_name, is_active)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (user_id) DO UPDATE SET
                username = EXCLUDED.username,
                team_name = EXCLUDED.team_name,
                is_active = EXCLUDED.is_active`,
			m.UserID, m.Username, t.TeamName, m.IsActive)
		if err != nil {
			return fmt.Errorf("insert/update user: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetTeam возвращает команду по её имени.
func (s *Store) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	team := &models.Team{TeamName: teamName}
	query := `
	SELECT user_id, username, is_active
	FROM users
	WHERE team_name = $1`
	rows, err := s.Pool.Query(ctx, query, teamName)
	if err != nil {
		return nil, fmt.Errorf("get team, Query: %w", err)
	}
	defer rows.Close()

	members := []models.TeamMember{}
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, fmt.Errorf("rows scan in getteam: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if len(members) == 0 {
		return nil, fmt.Errorf("GetTeam: %w", ErrTeamNotFound)
	}
	team.Members = members
	return team, err
}

// SetUserActive изменяет флаг активности пользователя
func (s *Store) SetUserActive(ctx context.Context, userID string, isActive bool) (*models.User, error) {
	var user models.User
	err := s.Pool.QueryRow(ctx, `
	UPDATE users
	SET is_active = $1
	WHERE user_id = $2
	RETURNING user_id, username, team_name, is_active`,
		isActive, userID).Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("set user active: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("set user active: %w", err)
	}
	return &user, nil
}

// CreatePR создает новый PR в базе данных.
func (s *Store) CreatePR(ctx context.Context, prID, prName, authorID string) (*models.PullRequest, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			log.Printf("Failed to rollback transaction: %v", err)
		}
	}()

	var exists bool
	err = tx.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)`,
		prID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check PR exists: %w", err)
	}
	if exists {
		return nil, ErrPRExists
	}

	var authorTeam string
	var authorExists bool
	err = tx.QueryRow(ctx, `
        SELECT team_name, true FROM users WHERE user_id = $1`,
		authorID).Scan(&authorTeam, &authorExists)

	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get author: %w", err)
	}

	rows, err := tx.Query(ctx, `
        SELECT user_id FROM users 
        WHERE team_name = $1 
        AND is_active = true 
        AND user_id != $2`,
		authorTeam, authorID)
	if err != nil {
		return nil, fmt.Errorf("get team members: %w", err)
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	reviewers := selectReviewers(candidates)

	pr := models.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            "OPEN",
		AssignedReviewers: reviewers,
		CreatedAt:         time.Now(),
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO pull_requests 
        (pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, pr.AssignedReviewers, pr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert PR: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &pr, nil
}

// MergePR помечает PR как мерженный.
func (s *Store) MergePR(ctx context.Context, prID string) (*models.PullRequest, error) {
	var pr models.PullRequest
	var mergedAt *time.Time

	err := s.Pool.QueryRow(ctx, `
		UPDATE pull_requests 
		SET status = 'MERGED', merged_at = COALESCE(merged_at, CURRENT_TIMESTAMP)
		WHERE pull_request_id = $1
		RETURNING pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at`,
		prID).Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status,
		&pr.AssignedReviewers, &pr.CreatedAt, &mergedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("merge PR: %w", err)
	}

	pr.MergedAt = mergedAt
	return &pr, nil
}

// ReassignReviewer переназначает ревьювера на PR.
func (s *Store) ReassignReviewer(ctx context.Context, prID, oldUserID string) (*models.PullRequest, string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			log.Printf("Failed to rollback transaction: %v", err)
		}
	}()

	var pr models.PullRequest
	var mergedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at
		FROM pull_requests WHERE pull_request_id = $1 FOR UPDATE`,
		prID).Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status,
		&pr.AssignedReviewers, &pr.CreatedAt, &mergedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("get PR: %w", err)
	}

	pr.MergedAt = mergedAt

	if pr.Status == "MERGED" {
		return nil, "", ErrPRMerged
	}

	if !contains(pr.AssignedReviewers, oldUserID) {
		return nil, "", ErrNotAssigned
	}

	rows, err := tx.Query(ctx, `
    SELECT u.user_id FROM users u
    WHERE u.team_name = (SELECT team_name FROM users WHERE user_id = $1)
    AND u.is_active = true
    AND u.user_id != $1
    AND u.user_id != (SELECT author_id FROM pull_requests WHERE pull_request_id = $2)
    AND NOT EXISTS (
        SELECT 1 FROM pull_requests pr 
        WHERE pr.pull_request_id = $2 
        AND pr.assigned_reviewers ? u.user_id
    )`,
		oldUserID, prID)
	if err != nil {
		return nil, "", fmt.Errorf("get candidates: %w", err)
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, "", fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("rows iteration: %w", err)
	}

	if len(candidates) == 0 {
		return nil, "", ErrNoCandidate
	}

	newUserID := candidates[0]

	newReviewers := replaceInSlice(pr.AssignedReviewers, oldUserID, newUserID)

	_, err = tx.Exec(ctx, `
		UPDATE pull_requests 
		SET assigned_reviewers = $1 
		WHERE pull_request_id = $2`,
		newReviewers, prID)
	if err != nil {
		return nil, "", fmt.Errorf("update reviewers: %w", err)
	}

	pr.AssignedReviewers = newReviewers

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("commit: %w", err)
	}

	return &pr, newUserID, nil
}

// GetUserReviews возвращает PR, где пользователь назначен ревьювером.
func (s *Store) GetUserReviews(ctx context.Context, userID string) ([]models.PullRequest, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT pull_request_id, pull_request_name, author_id, status
		FROM pull_requests 
		WHERE assigned_reviewers ? $1`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("get user reviews: %w", err)
	}
	defer rows.Close()

	var prs []models.PullRequest
	for rows.Next() {
		var pr models.PullRequest
		if err := rows.Scan(
			&pr.PullRequestID,
			&pr.PullRequestName,
			&pr.AuthorID,
			&pr.Status,
		); err != nil {
			return nil, fmt.Errorf("scan PR: %w", err)
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

func selectReviewers(candidates []string) []string {
	if len(candidates) == 0 {
		return []string{}
	}
	if len(candidates) <= 2 {
		return candidates
	}

	shuffled := make([]string, len(candidates))
	copy(shuffled, candidates)

	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	count := minInt(2, len(shuffled))
	return shuffled[:count]
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func replaceInSlice(slice []string, old, replacement string) []string {
	result := make([]string, len(slice))
	for i, v := range slice {
		if v == old {
			result[i] = replacement
		} else {
			result[i] = v
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetStats возвращает статистику сервиса
func (s *Store) GetStats(ctx context.Context) (*models.Stats, error) {
	stats := &models.Stats{
		PRsByStatus:   make(map[string]int),
		ReviewsByUser: make(map[string]int),
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT status, COUNT(*) 
		FROM pull_requests 
		GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("get PR stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan PR stats: %w", err)
		}
		stats.PRsByStatus[status] = count
		stats.TotalPRs += count

	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	rows, err = s.Pool.Query(ctx, `
		SELECT reviewer, COUNT(*) 
		FROM pull_requests, jsonb_array_elements_text(assigned_reviewers) AS reviewer
		GROUP BY reviewer
		ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, fmt.Errorf("get reviews stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return nil, fmt.Errorf("scan reviews stats: %w", err)
		}
		stats.ReviewsByUser[userID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	err = s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM users 
		WHERE is_active = true`).Scan(&stats.ActiveUsers)
	if err != nil {
		return nil, fmt.Errorf("get active users: %w", err)
	}

	err = s.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT team_name) 
		FROM users`).Scan(&stats.TotalTeams)
	if err != nil {
		return nil, fmt.Errorf("get teams count: %w", err)
	}

	return stats, nil
}
