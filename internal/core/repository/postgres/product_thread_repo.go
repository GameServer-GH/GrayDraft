package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type ProductThread struct {
	ThreadKey string
	Payload   []byte
	UpdatedAt time.Time
}

type ProductThreadRepo struct{ db *Client }

func NewProductThreadRepo(db *Client) *ProductThreadRepo { return &ProductThreadRepo{db: db} }

func (r *ProductThreadRepo) Get(ctx context.Context, tenantID, actorID, threadKey string) (ProductThread, error) {
	var out ProductThread
	err := r.db.pool.QueryRow(ctx, `SELECT thread_key, payload, updated_at FROM product_threads WHERE tenant_id=$1 AND actor_id=$2 AND thread_key=$3`, tenantID, actorID, threadKey).Scan(&out.ThreadKey, &out.Payload, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductThread{}, err
	}
	if err != nil {
		return ProductThread{}, fmt.Errorf("get product thread: %w", err)
	}
	return out, nil
}

func (r *ProductThreadRepo) Put(ctx context.Context, tenantID, actorID, threadKey string, payload []byte) error {
	_, err := r.db.pool.Exec(ctx, `INSERT INTO product_threads(tenant_id, actor_id, thread_key, payload, updated_at)
		VALUES($1,$2,$3,$4::jsonb,now())
		ON CONFLICT(tenant_id, actor_id, thread_key)
		DO UPDATE SET payload=EXCLUDED.payload, updated_at=now()`, tenantID, actorID, threadKey, payload)
	if err != nil {
		return fmt.Errorf("put product thread: %w", err)
	}
	return nil
}

func (r *ProductThreadRepo) Delete(ctx context.Context, tenantID, actorID, threadKey string) error {
	_, err := r.db.pool.Exec(ctx, `DELETE FROM product_threads WHERE tenant_id=$1 AND actor_id=$2 AND thread_key=$3`, tenantID, actorID, threadKey)
	if err != nil {
		return fmt.Errorf("delete product thread: %w", err)
	}
	return nil
}

func (r *ProductThreadRepo) DeleteAll(ctx context.Context, tenantID, actorID string) error {
	_, err := r.db.pool.Exec(ctx, `DELETE FROM product_threads WHERE tenant_id=$1 AND actor_id=$2`, tenantID, actorID)
	if err != nil {
		return fmt.Errorf("delete product threads: %w", err)
	}
	return nil
}
