package store

import "errors"

var (
	// ErrTeamNotFound возвращается когда команда не найдена.
	ErrTeamNotFound = errors.New("team not found")

	// ErrTeamExists возвращается когда команда уже существует.
	ErrTeamExists = errors.New("team already exists")

	// ErrPRExists возвращается когда PR уже существует.
	ErrPRExists = errors.New("PR exists")

	// ErrPRMerged возвращается когда операция недопустима для мерженного PR.
	ErrPRMerged = errors.New("PR merged")

	// ErrNotAssigned возвращается когда пользователь не назначен ревьювером.
	ErrNotAssigned = errors.New("reviewer not assigned")

	// ErrNoCandidate возвращается когда нет кандидатов для переназначения.
	ErrNoCandidate = errors.New("no candidate")

	// ErrNotFound возвращается когда ресурс не найден.
	ErrNotFound = errors.New("not found")
)
