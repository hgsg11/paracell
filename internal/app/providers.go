package app

import (
	"context"

	"github.com/shige1114/paradev/internal/domain"
)

type staticConfig struct {
	cfg domain.Config
}

func (s staticConfig) Load(ctx context.Context) (domain.Config, error) {
	return s.cfg, nil
}
