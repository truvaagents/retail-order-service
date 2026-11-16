package main

import (
	"net/http/httptest"
	"testing"
)

func TestAuthorizationHeaderSchemes(t *testing.T) {
	const testToken = "synthetic-test-token-for-auth-checks"
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "matching bearer", header: "Bearer " + testToken, want: true},
		{name: "wrong token", header: "Bearer a-different-synthetic-test-token", want: false},
		{name: "missing header", header: "", want: false},
		{name: "different scheme", header: "Basic " + testToken, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/orders", nil)
			request.Header.Set("Authorization", test.header)
			if got := Authorized(request, testToken); got != test.want {
				t.Fatalf("Authorized() = %t, want %t", got, test.want)
			}
		})
	}
}
