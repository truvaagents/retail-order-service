package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func Authorized(r *http.Request, token string) bool {
	provided := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	expected := sha256.Sum256([]byte("Bearer " + token))
	return len(token) >= 32 && subtle.ConstantTimeCompare(provided[:], expected[:]) == 1
}

func ReadJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid request body")
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) {
		return errors.New("expected one JSON value")
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "response encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
