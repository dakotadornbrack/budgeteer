package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/google/uuid"
	"github.com/dakotadornbrack/budgeteer/internal/model"
)

var ErrNotFound = errors.New("not found")

// Store defines the persistence interface.
// Swap PostgresStore for any other implementation without touching handlers.
type Store interface {
	CreateTransaction(input *model.CreateTransactionInput) (*model.Transaction, error)
	GetTransaction(id string) (*model.Transaction, error)
	ListTransactions(filter model.TransactionFilter) ([]*model.Transaction, error)
	DeleteTransaction(id string) error

	SetBudget(input *model.CreateBudgetInput) (*model.Budget, error)
	GetBudget(category model.Category, month, year int) (*model.Budget, error)
	ListBudgets(month, year int) ([]*model.Budget, error)

	GetSummary(month, year int) ([]*model.Summary, error)
}

// PostgresStore is the production Store backed by PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) CreateTransaction(input *model.CreateTransactionInput) (*model.Transaction, error) {
	t := &model.Transaction{
		ID:          uuid.New().String(),
		Description: input.Description,
		AmountCents: input.AmountCents,
		Category:    input.Category,
		Date:        input.Date,
		CreatedAt:   time.Now().UTC(),
	}

	_, err := s.db.Exec(`
		INSERT INTO transactions (id, description, amount_cents, category, date, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.Description, t.AmountCents, string(t.Category), t.Date, t.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting transaction: %w", err)
	}
	return t, nil
}

func (s *PostgresStore) GetTransaction(id string) (*model.Transaction, error) {
	row := s.db.QueryRow(`
		SELECT id, description, amount_cents, category, date, created_at
		FROM transactions WHERE id = $1`, id)

	t := &model.Transaction{}
	var category string
	err := row.Scan(&t.ID, &t.Description, &t.AmountCents, &category, &t.Date, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning transaction: %w", err)
	}
	t.Category = model.Category(category)
	return t, nil
}

func (s *PostgresStore) ListTransactions(filter model.TransactionFilter) ([]*model.Transaction, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, description, amount_cents, category, date, created_at
		FROM transactions WHERE 1=1`
	args := []any{}
	n := 1

	if filter.Category != "" {
		query += fmt.Sprintf(" AND category = $%d", n)
		args = append(args, string(filter.Category))
		n++
	}
	if filter.Month > 0 {
		query += fmt.Sprintf(" AND EXTRACT(MONTH FROM date) = $%d", n)
		args = append(args, filter.Month)
		n++
	}
	if filter.Year > 0 {
		query += fmt.Sprintf(" AND EXTRACT(YEAR FROM date) = $%d", n)
		args = append(args, filter.Year)
		n++
	}

	query += fmt.Sprintf(" ORDER BY date DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, filter.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying transactions: %w", err)
	}
	defer rows.Close()

	var results []*model.Transaction
	for rows.Next() {
		t := &model.Transaction{}
		var category string
		if err := rows.Scan(&t.ID, &t.Description, &t.AmountCents, &category, &t.Date, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		t.Category = model.Category(category)
		results = append(results, t)
	}
	return results, rows.Err()
}

func (s *PostgresStore) DeleteTransaction(id string) error {
	res, err := s.db.Exec(`DELETE FROM transactions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting transaction: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SetBudget(input *model.CreateBudgetInput) (*model.Budget, error) {
	b := &model.Budget{
		ID:         uuid.New().String(),
		Category:   input.Category,
		LimitCents: input.LimitCents,
		Month:      input.Month,
		Year:       input.Year,
		CreatedAt:  time.Now().UTC(),
	}

	// Upsert: if a budget already exists for this category+month+year, update it.
	_, err := s.db.Exec(`
		INSERT INTO budgets (id, category, limit_cents, month, year, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (category, month, year) DO UPDATE SET limit_cents = EXCLUDED.limit_cents`,
		b.ID, string(b.Category), b.LimitCents, b.Month, b.Year, b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting budget: %w", err)
	}
	return b, nil
}

func (s *PostgresStore) GetBudget(category model.Category, month, year int) (*model.Budget, error) {
	row := s.db.QueryRow(`
		SELECT id, category, limit_cents, month, year, created_at
		FROM budgets WHERE category = $1 AND month = $2 AND year = $3`,
		string(category), month, year)

	b := &model.Budget{}
	var cat string
	err := row.Scan(&b.ID, &cat, &b.LimitCents, &b.Month, &b.Year, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning budget: %w", err)
	}
	b.Category = model.Category(cat)
	return b, nil
}

func (s *PostgresStore) ListBudgets(month, year int) ([]*model.Budget, error) {
	rows, err := s.db.Query(`
		SELECT id, category, limit_cents, month, year, created_at
		FROM budgets WHERE month = $1 AND year = $2`, month, year)
	if err != nil {
		return nil, fmt.Errorf("querying budgets: %w", err)
	}
	defer rows.Close()

	var results []*model.Budget
	for rows.Next() {
		b := &model.Budget{}
		var cat string
		if err := rows.Scan(&b.ID, &cat, &b.LimitCents, &b.Month, &b.Year, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning budget row: %w", err)
		}
		b.Category = model.Category(cat)
		results = append(results, b)
	}
	return results, rows.Err()
}

func (s *PostgresStore) GetSummary(month, year int) ([]*model.Summary, error) {
	// Join transactions with budgets to produce per-category spend vs limit.
	rows, err := s.db.Query(`
		SELECT
			t.category,
			COALESCE(SUM(t.amount_cents), 0) AS total_cents,
			COALESCE(b.limit_cents, 0)        AS budget_cents
		FROM transactions t
		LEFT JOIN budgets b
			ON t.category = b.category AND b.month = $1 AND b.year = $2
		WHERE EXTRACT(MONTH FROM t.date) = $1
		  AND EXTRACT(YEAR  FROM t.date) = $2
		GROUP BY t.category, b.limit_cents
		ORDER BY total_cents DESC`,
		month, year)
	if err != nil {
		return nil, fmt.Errorf("querying summary: %w", err)
	}
	defer rows.Close()

	var results []*model.Summary
	for rows.Next() {
		s := &model.Summary{Month: month, Year: year}
		var cat string
		if err := rows.Scan(&cat, &s.TotalCents, &s.BudgetCents); err != nil {
			return nil, fmt.Errorf("scanning summary row: %w", err)
		}
		s.Category = model.Category(cat)
		results = append(results, s)
	}
	return results, rows.Err()
}
