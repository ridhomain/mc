package router

import (
	"mailcast-service-v2/internal/usecase"
	"mailcast-service-v2/pkg/logger"
)

// SubjectRouter routes emails to appropriate handlers based on subject
type SubjectRouter struct {
	handlers []usecase.TemplateHandler
	logger   logger.Logger
}

// NewSubjectRouter creates a new subject router
func NewSubjectRouter(logger logger.Logger) *SubjectRouter {
	return &SubjectRouter{
		handlers: make([]usecase.TemplateHandler, 0),
		logger:   logger,
	}
}

// Register registers a handler for specific subject patterns
func (r *SubjectRouter) Register(handler usecase.TemplateHandler) {
	r.handlers = append(r.handlers, handler)
	r.logger.Info("Registered handler", "handler", handler)
}

// GetHandler returns the appropriate handler for a given subject
func (r *SubjectRouter) GetHandler(subject string) usecase.TemplateHandler {
	for _, handler := range r.handlers {
		if handler.CanHandle(subject) {
			return handler
		}
	}
	return nil
}
