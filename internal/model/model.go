package model

import (
	"errors"
	"time"
)

type Category string

const (
	CategoryFood          Category = "food"
	CategoryTransport     Category = "transport"
	CategoryHousing       Category = "housing"
	CategoryEntertainment Category = "entertainment"
	CategoryHealth        Category = "health"
	CategoryOther         Category = "other"
)

// Transaction is the core domain object.
type Transaction struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	AmountCents int64     `json:"amount_cents"` // stored in cents to avoid float precision issues
	Category    Category  `json:"category"`
	Date        time.Time `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

// Budget represents a monthly spending limit for a category.
type Budget struct {
	ID          string    `json:"id"`
	Category    Category  `json:"category"`
	LimitCents  int64     `json:"limit_cents"`
	Month       int       `json:"month"` // 1-12
	Year        int       `json:"year"`
	CreatedAt   time.Time `json:"created_at"`
}

// Summary is a computed aggregate — not stored, derived on read.
type Summary struct {
	Category    Category `json:"category"`
	TotalCents  int64    `json:"total_cents"`
	BudgetCents int64    `json:"budget_cents"` // 0 if no budget set
	Month       int      `json:"month"`
	Year        int      `json:"year"`
}

// CreateTransactionInput is the input for creating a transaction.
type CreateTransactionInput struct {
	Description string
	AmountCents int64
	Category    Category
	Date        time.Time
}

func (i *CreateTransactionInput) Validate() error {
	if i.Description == "" {
		return errors.New("description is required")
	}
	if i.AmountCents <= 0 {
		return errors.New("amount_cents must be greater than zero")
	}
	switch i.Category {
	case CategoryFood, CategoryTransport, CategoryHousing,
		CategoryEntertainment, CategoryHealth, CategoryOther:
	case "":
		return errors.New("category is required")
	default:
		return errors.New("invalid category")
	}
	return nil
}

// CreateBudgetInput is the input for setting a monthly budget.
type CreateBudgetInput struct {
	Category   Category
	LimitCents int64
	Month      int
	Year       int
}

func (i *CreateBudgetInput) Validate() error {
	if i.LimitCents <= 0 {
		return errors.New("limit_cents must be greater than zero")
	}
	if i.Month < 1 || i.Month > 12 {
		return errors.New("month must be between 1 and 12")
	}
	if i.Year < 2000 || i.Year > 2100 {
		return errors.New("year is out of range")
	}
	return nil
}

// TransactionFilter allows optional filtering when listing transactions.
type TransactionFilter struct {
	Category Category
	Month    int
	Year     int
	Limit    int
	Offset   int
}
