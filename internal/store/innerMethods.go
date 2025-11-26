package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/2Empty/review-assigner/internal/models"
	"github.com/jackc/pgx/v5"
)

func deactivateUserInTx(ctx context.Context, tx pgx.Tx, userID string) (*models.User, error) {
	// Пытаемся переназначить ревьюеров для открытых PR
	prs, err := getUserReviewsInTx(ctx, tx, userID)
	if err != nil {
		// Если не удалось получить список, всё равно деактивируем пользователя
		log.Printf("Failed to get user reviews for reassignment: %v", err)
	} else {
		for _, pr := range prs {
			if pr.Status == "OPEN" {
				_, _, err := reassignReviewerInTx(ctx, tx, pr.PullRequestID, userID)
				if err != nil {
					// Логируем, но продолжаем - не критично если не удалось переназначить
					log.Printf("Failed to reassign reviewer for PR %s: %v", pr.PullRequestID, err)
				}
			}
		}
	}

	// Деактивируем пользователя
	var user models.User
	err = tx.QueryRow(ctx, `
		UPDATE users
		SET is_active = false
		WHERE user_id = $1
		RETURNING user_id, username, team_name, is_active`,
		userID).Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("set user active: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("set user active: %w", err)
	}
	return &user, nil
}

func getUserReviewsInTx(ctx context.Context, tx pgx.Tx, userID string) ([]models.PullRequest, error) {
	rows, err := tx.Query(ctx, `
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

func reassignReviewerInTx(ctx context.Context, tx pgx.Tx, prID, oldUserID string) (*models.PullRequest, string, error) {
	var pr models.PullRequest
	var mergedAt *time.Time
	err := tx.QueryRow(ctx, `
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

	return &pr, newUserID, nil
}
