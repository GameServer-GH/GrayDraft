package postgres

import (
	"encoding/json"

	"github.com/graydraft/core/model"
)

func parseExtensions(raw []byte) model.Extensions {
	if len(raw) == 0 {
		return nil
	}
	var m model.Extensions
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return nil
	}
	return m
}
