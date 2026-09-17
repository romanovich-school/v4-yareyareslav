package request

import (
	"encoding/json"
	"io"
	"net/http"
)

func Body[T any](w http.ResponseWriter, r *http.Request) (result T, err error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(body), &result);
	if err != nil {
		http.Error(w, "failed to read unmarshal body", http.StatusBadRequest)
		return 
	}
	return
}