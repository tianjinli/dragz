// Package svc exposes the service context the logic layer consumes.
package svc

import "dragz-admin/backend/internal/app"

// ServiceContext mirrors Simple Admin api/internal/svc and wraps the app runtime.
type ServiceContext struct {
	*app.Context
}

// NewServiceContext adapts the application context to the service-context shape the logic layer expects.
func NewServiceContext(ctx *app.Context) *ServiceContext {
	return &ServiceContext{Context: ctx}
}
