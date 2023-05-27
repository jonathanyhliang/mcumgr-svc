package main

import (
	"context"
	"errors"
	"os"
)

var (
	ErrImageNotfound = errors.New("download image failed")
)

type Status struct {
	Execution string `json:"execution"`
	Result    struct {
		Finished string `json:"finished"`
	} `json:"result"`
}

type Image struct {
	File string `json:"file"`
}

type Service interface {
	UploadImage(ctx context.Context, img Image) error
	GetStatus(ctx context.Context) (Status, error)
	Reset(ctx context.Context) error
}

type McumgrService struct{}

func NewMcumgrService() Service {
	return &McumgrService{}
}

func (m *McumgrService) UploadImage(ctx context.Context, img Image) error {
	f, err := os.Open(img.File)
	if err != nil {
		return ErrImageNotfound
	}
	defer f.Close()
	return nil
}

func (m *McumgrService) GetStatus(ctx context.Context) (Status, error) {
	return Status{}, nil
}

func (m *McumgrService) Reset(ctx context.Context) error {
	return nil
}
