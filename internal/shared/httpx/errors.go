package httpx

import (
	"errors"
	"log"
	"net/http"
)

type Error struct {
	Status  int
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func Errorf(status int, message string) *Error {
	return &Error{Status: status, Message: message}
}

func Wrapf(status int, message string, err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Status: status, Message: message, Err: err}
}

func WriteError(w http.ResponseWriter, err error) {
	var e *Error
	if errors.As(err, &e) {
		if e.Status >= http.StatusInternalServerError {
			log.Printf("%s: %v", e.Message, err)
		}
		http.Error(w, e.Message, e.Status)
		return
	}

	log.Printf("erro não mapeado: %v", err)
	http.Error(w, "Erro interno", http.StatusInternalServerError)
}
