package shikirequests

import (
	"fmt"
	"net/http"
	// "smOwd2/logs"
)

func HandleAuthCode(w http.ResponseWriter, r *http.Request) (string, error) {
	code := r.URL.Query().Get("code")

	if code == "" {
		errMsg := "Auth code not provided"
		return "", fmt.Errorf(errMsg)
	}

	return code, nil
}
