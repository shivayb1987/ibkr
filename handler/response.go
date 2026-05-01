package handler

import (
	"github.com/go-chi/render"
	"net/http"
)

const (
	statusError   = "ERROR"
	statusSuccess = "SUCCESS"
)

type Response struct {
	Code    int         `json:"code"`
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(w http.ResponseWriter, r *http.Request, data interface{}) {
	render.Status(r, http.StatusOK)
	render.JSON(w, r, &Response{
		Code:   http.StatusOK,
		Status: statusSuccess,
		Data:   data,
	})
}

func Errors(w http.ResponseWriter, r *http.Request, httpStatus int, message string) {
	render.Status(r, httpStatus)

	render.JSON(w, r, &Response{
		Code:    httpStatus,
		Status:  statusError,
		Message: message,
	})
}
