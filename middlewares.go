package main

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
)

// Middleware describes a service (as opposed to endpoint) middleware.
type Middleware func(Service) Service

func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) UploadImage(ctx context.Context, img Image) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "UploadImage", "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.UploadImage(ctx, img)
}

func (mw loggingMiddleware) GetStatus(ctx context.Context) (status Status, err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "GetStatus", "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.GetStatus(ctx)
}

func (mw loggingMiddleware) Reset(ctx context.Context) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "Reset", "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.Reset(ctx)
}

func BackendMiddleware(backend Backend) Middleware {
	return func(next Service) Service {
		return &backendMiddleware{
			next:    next,
			backend: backend,
		}
	}
}

type backendMiddleware struct {
	next    Service
	backend Backend
}

func (mw backendMiddleware) UploadImage(ctx context.Context, img Image) (err error) {
	e := mw.next.UploadImage(ctx, img)
	if e == nil {
		mw.backend.UploadImage(img.File)
		return nil
	}
	return e
}

func (mw backendMiddleware) GetStatus(ctx context.Context) (status Status, err error) {
	_, _ = mw.next.GetStatus(ctx)
	s := mw.backend.GetStatus()
	return s, nil
}

func (mw backendMiddleware) Reset(ctx context.Context) (err error) {
	_ = mw.next.Reset(ctx)
	mw.backend.Reset()
	return nil
}
