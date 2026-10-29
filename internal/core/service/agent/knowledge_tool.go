package agent

import (
	"context"

	"github.com/graydraft/core/model"
)

// Searcher is the retrieval boundary used by the education tools.
type Searcher interface {
	Search(ctx context.Context, req model.SearchRequest) (*model.SearchResponse, error)
}
