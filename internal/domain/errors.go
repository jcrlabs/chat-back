package domain

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrRoomFull   = errors.New("room is full")
	ErrBadRequest = errors.New("bad request")
)
