package json

import (
	"encoding/json"
	"fmt"

	"github.com/arl/gitstatus"
)

type Formater struct{}

func (Formater) Format(st *gitstatus.Status) (string, error) {
	buf, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return "", fmt.Errorf("can't format status to json: %v", err)
	}
	return string(buf), nil
}
