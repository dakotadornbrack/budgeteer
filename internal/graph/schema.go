package graph

import (
	"fmt"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/dakotadornbrack/budgeteer/internal/cache"
	"github.com/dakotadornbrack/budgeteer/internal/model"
	"github.com/dakotadornbrack/budgeteer/internal/store"
)

// Build constructs the full GraphQL schema.
func Build(s *cache.CachedStore) (graphql.Schema, error) {

	// ── Enums ──────────────────────────────────────────────────────────────

	categoryEnum := graphql.NewEnum(graphql.EnumConfig{
		Name: "Category",
		Values: graphql.EnumValueConfigMap{
			"food":          {Value: model.CategoryFood},
			"transport":     {Value: model.CategoryTransport},
			"housing":       {Value: model.CategoryHousing},
			"entertainment": {Value: model.CategoryEntertainment},
			"health":        {Value: model.CategoryHealth},
			"other":         {Value: model.CategoryOther},
		},
	})

	// ── Types ──────────────────────────────────────────────────────────────

	transactionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Transaction",
		Fields: graphql.Fields{
			"id":          {Type: graphql.NewNonNull(graphql.String)},
			"description": {Type: graphql.NewNonNull(graphql.String)},
			"amountCents": {
				Type: graphql.NewNonNull(graphql.Int),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return int(p.Source.(*model.Transaction).AmountCents), nil
				},
			},
			"category": {Type: graphql.NewNonNull(categoryEnum)},
			"date": {
				Type: graphql.NewNonNull(graphql.String),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(*model.Transaction).Date.Format(time.DateOnly), nil
				},
			},
			"createdAt": {
				Type: graphql.NewNonNull(graphql.String),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(*model.Transaction).CreatedAt.Format(time.RFC3339), nil
				},
			},
		},
	})

	budgetType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Budget",
		Fields: graphql.Fields{
			"id":       {Type: graphql.NewNonNull(graphql.String)},
			"category": {Type: graphql.NewNonNull(categoryEnum)},
			"limitCents": {
				Type: graphql.NewNonNull(graphql.Int),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return int(p.Source.(*model.Budget).LimitCents), nil
				},
			},
			"month": {Type: graphql.NewNonNull(graphql.Int)},
			"year":  {Type: graphql.NewNonNull(graphql.Int)},
		},
	})

	summaryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Summary",
		Fields: graphql.Fields{
			"category": {Type: graphql.NewNonNull(categoryEnum)},
			"totalCents": {
				Type: graphql.NewNonNull(graphql.Int),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return int(p.Source.(*model.Summary).TotalCents), nil
				},
			},
			"budgetCents": {
				Type: graphql.NewNonNull(graphql.Int),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return int(p.Source.(*model.Summary).BudgetCents), nil
				},
			},
			"month": {Type: graphql.NewNonNull(graphql.Int)},
			"year":  {Type: graphql.NewNonNull(graphql.Int)},
		},
	})

	// ── Query ─────────────────────────────────────────────────────────────

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{

			"transaction": {
				Type:        transactionType,
				Description: "Fetch a single transaction by ID.",
				Args: graphql.FieldConfigArgument{
					"id": {Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					t, err := s.GetTransaction(p.Args["id"].(string))
					if err != nil {
						return nil, err
					}
					return t, nil
				},
			},

			"transactions": {
				Type:        graphql.NewList(transactionType),
				Description: "List transactions with optional filters.",
				Args: graphql.FieldConfigArgument{
					"category": {Type: categoryEnum},
					"month":    {Type: graphql.Int},
					"year":     {Type: graphql.Int},
					"limit":    {Type: graphql.Int},
					"offset":   {Type: graphql.Int},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					filter := model.TransactionFilter{}
					if v, ok := p.Args["category"].(model.Category); ok {
						filter.Category = v
					}
					if v, ok := p.Args["month"].(int); ok {
						filter.Month = v
					}
					if v, ok := p.Args["year"].(int); ok {
						filter.Year = v
					}
					if v, ok := p.Args["limit"].(int); ok {
						filter.Limit = v
					}
					if v, ok := p.Args["offset"].(int); ok {
						filter.Offset = v
					}
					return s.ListTransactions(filter)
				},
			},

			"budgets": {
				Type:        graphql.NewList(budgetType),
				Description: "List budgets for a given month and year.",
				Args: graphql.FieldConfigArgument{
					"month": {Type: graphql.NewNonNull(graphql.Int)},
					"year":  {Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return s.ListBudgets(p.Args["month"].(int), p.Args["year"].(int))
				},
			},

			"summary": {
				Type:        graphql.NewList(summaryType),
				Description: "Spending vs budget summary by category. Results are cached in Redis for 5 minutes.",
				Args: graphql.FieldConfigArgument{
					"month": {Type: graphql.NewNonNull(graphql.Int)},
					"year":  {Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return s.GetSummary(p.Args["month"].(int), p.Args["year"].(int))
				},
			},
		},
	})

	// ── Mutation ──────────────────────────────────────────────────────────

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{

			"createTransaction": {
				Type:        transactionType,
				Description: "Record a new transaction.",
				Args: graphql.FieldConfigArgument{
					"description": {Type: graphql.NewNonNull(graphql.String)},
					"amountCents": {Type: graphql.NewNonNull(graphql.Int)},
					"category":    {Type: graphql.NewNonNull(categoryEnum)},
					"date":        {Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					date := time.Now().UTC()
					if d, ok := p.Args["date"].(string); ok && d != "" {
						parsed, err := time.Parse(time.DateOnly, d)
						if err != nil {
							return nil, fmt.Errorf("date must be YYYY-MM-DD: %w", err)
						}
						date = parsed
					}

					input := &model.CreateTransactionInput{
						Description: p.Args["description"].(string),
						AmountCents: int64(p.Args["amountCents"].(int)),
						Category:    p.Args["category"].(model.Category),
						Date:        date,
					}
					if err := input.Validate(); err != nil {
						return nil, err
					}

					t, err := s.CreateTransaction(input)
					if err != nil {
						return nil, err
					}

					// Invalidate cached summary for the transaction's month/year
					s.InvalidateSummary(t.Date.Year(), int(t.Date.Month()))
					return t, nil
				},
			},

			"deleteTransaction": {
				Type:        graphql.Boolean,
				Description: "Delete a transaction by ID.",
				Args: graphql.FieldConfigArgument{
					"id": {Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					t, err := s.GetTransaction(p.Args["id"].(string))
					if err != nil {
						return false, err
					}
					if err := s.DeleteTransaction(p.Args["id"].(string)); err != nil {
						return false, err
					}
					s.InvalidateSummary(t.Date.Year(), int(t.Date.Month()))
					return true, nil
				},
			},

			"setBudget": {
				Type:        budgetType,
				Description: "Set or update a monthly budget for a category.",
				Args: graphql.FieldConfigArgument{
					"category":   {Type: graphql.NewNonNull(categoryEnum)},
					"limitCents": {Type: graphql.NewNonNull(graphql.Int)},
					"month":      {Type: graphql.NewNonNull(graphql.Int)},
					"year":       {Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					input := &model.CreateBudgetInput{
						Category:   p.Args["category"].(model.Category),
						LimitCents: int64(p.Args["limitCents"].(int)),
						Month:      p.Args["month"].(int),
						Year:       p.Args["year"].(int),
					}
					if err := input.Validate(); err != nil {
						return nil, err
					}
					b, err := s.SetBudget(input)
					if err != nil {
						return nil, err
					}
					s.InvalidateSummary(b.Month, b.Year)
					return b, nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
}

// Ensure PostgresStore satisfies the Store interface at compile time.
var _ store.Store = (*store.PostgresStore)(nil)
