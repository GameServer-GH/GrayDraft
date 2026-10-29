package agent

import (
	"context"
	"errors"
	"fmt"
)

// ErrApprovalRequired indicates an R2 tool call needs human approval.
var ErrApprovalRequired = errors.New("approval required")

// GlobalBudgetGuard enforces tenant-wide daily limits before starting a Run.
type GlobalBudgetGuard struct {
	DailyTokens func(ctx context.Context, tenantID string) (int64, error)
	DailyRuns   func(ctx context.Context, tenantID string) (int, error)
	Policy      func(ctx context.Context, tenantID string) (BudgetPolicy, error)
}

type BudgetPolicy struct {
	DailyTokenLimit      int64
	DailyRunLimit        int
	CollaborativeEnabled bool
}

func (g GlobalBudgetGuard) Check(ctx context.Context, tenantID string, collaborative bool) error {
	if g.Policy == nil {
		return nil
	}
	p, err := g.Policy(ctx, tenantID)
	if err != nil {
		return err
	}
	if collaborative && !p.CollaborativeEnabled {
		return fmt.Errorf("agent: collaborative runs disabled for tenant")
	}
	if p.DailyRunLimit > 0 && g.DailyRuns != nil {
		n, err := g.DailyRuns(ctx, tenantID)
		if err != nil {
			return err
		}
		if n >= p.DailyRunLimit {
			return fmt.Errorf("agent: daily run limit exceeded")
		}
	}
	if p.DailyTokenLimit > 0 && g.DailyTokens != nil {
		tokens, err := g.DailyTokens(ctx, tenantID)
		if err != nil {
			return err
		}
		if tokens >= p.DailyTokenLimit {
			return fmt.Errorf("agent: daily token budget exceeded")
		}
	}
	return nil
}
