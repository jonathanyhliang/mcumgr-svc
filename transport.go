package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
)

var (
	// ErrBadRouting is returned when an expected path variable is missing.
	// It always indicates programmer error.
	ErrBadRouting = errors.New("inconsistent mapping between route and handler (programmer error)")
)

func MakeHTTPHandler(s Service, logger log.Logger) http.Handler {
	r := mux.NewRouter()
	e := MakeServerEndpoints(s)
	options := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(transport.NewLogErrorHandler(logger)),
		httptransport.ServerErrorEncoder(encodeError),
	}

	// POST	/mcumgr/upload		ploads image via MCUMgr
	// GET	/mcumgr/status		retrieves image uploading status via MCUMgr
	// POST /mcumgr/reset     	trigers cold reset

	r.Methods("POST").Path("/mcumgr/upload").Handler(httptransport.NewServer(
		e.UploadImage,
		decodeUploadImageEndpoint,
		encodeResponse,
		options...,
	))
	r.Methods("GET").Path("/mcumgr/status").Handler(httptransport.NewServer(
		e.GetStatus,
		decodeGetStatusEndpoint,
		encodeResponse,
		options...,
	))
	r.Methods("POST").Path("/mcumgr/reset").Handler(httptransport.NewServer(
		e.Reset,
		decodeResetEndpoint,
		encodeResponse,
		options...,
	))
	return r
}

func decodeUploadImageEndpoint(_ context.Context, r *http.Request) (request interface{}, err error) {
	var img Image
	if e := json.NewDecoder(r.Body).Decode(&img); e != nil {
		return nil, e
	}
	return uploadImageRequest{Img: img}, nil
}

func decodeGetStatusEndpoint(_ context.Context, r *http.Request) (request interface{}, err error) {
	return getStatusRequest{}, nil
}

func decodeResetEndpoint(_ context.Context, r *http.Request) (request interface{}, err error) {
	return resetRequest{}, nil
}

type errorer interface {
	error() error
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if e, ok := response.(errorer); ok && e.error() != nil {
		// Not a Go kit transport error, but a business-logic error.
		// Provide those as HTTP errors.
		encodeError(ctx, e.error(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codeFrom(err))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func codeFrom(err error) int {
	switch err {
	case ErrImageNotfound:
		return http.StatusNotFound
	// case ErrDownloadImage:
	// 	return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
