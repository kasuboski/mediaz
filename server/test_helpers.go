package server

import (
	"github.com/go-playground/validator/v10"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"go.uber.org/zap"
)

// newTestServer returns a Server with a validator and nop logger for testing.
// Use this instead of constructing Server{} directly to ensure the validator
// is always initialized.
func newTestServer(opts ...func(*Server)) Server {
	s := Server{
		baseLogger: zap.NewNop().Sugar(),
		validate:   validator.New(validator.WithRequiredStructEnabled()),
	}
	for _, o := range opts {
		o(&s)
	}
	return s
}

func withManager(mgr manager.MediaManager) func(*Server) {
	return func(s *Server) { s.manager = mgr }
}

func withConfig(cfg config.Server) func(*Server) {
	return func(s *Server) { s.config = cfg }
}
