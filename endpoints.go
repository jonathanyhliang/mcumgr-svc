package main

import (
	"context"

	"github.com/go-kit/kit/endpoint"
)

type Endpoints struct {
	UploadImage endpoint.Endpoint
	GetStatus   endpoint.Endpoint
	Reset       endpoint.Endpoint
}

func MakeServerEndpoints(s Service) Endpoints {
	return Endpoints{
		UploadImage: MakeUploadImage(s),
		GetStatus:   MakeGetStatus(s),
		Reset:       MakeReset(s),
	}
}

func MakeUploadImage(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(uploadImageRequest)
		e := s.UploadImage(ctx, req.Img)
		return uploadImageResponse{Err: e}, nil
	}
}

func MakeGetStatus(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		_ = request.(getStatusRequest)
		s, e := s.GetStatus(ctx)
		return getStatusResponse{Sta: s, Err: e}, nil
	}
}

func MakeReset(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		_ = request.(resetRequest)
		e := s.Reset(ctx)
		return resetResponse{Err: e}, nil
	}
}

type uploadImageRequest struct {
	Img Image
}

type uploadImageResponse struct {
	Err error `json:"err,omitempty"`
}

func (r uploadImageResponse) error() error { return r.Err }

type getStatusRequest struct{}

type getStatusResponse struct {
	Sta Status `json:"status,omitempty"`
	Err error  `json:"err,omitempty"`
}

func (r getStatusResponse) error() error { return r.Err }

type resetRequest struct{}

type resetResponse struct {
	Err error `json:"err,omitempty"`
}

func (r resetResponse) error() error { return r.Err }
