package game

import (
	"encoding/json"

	"github.com/google/uuid"
)

func GenID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic("Failed to generate UUID: " + err.Error())
	}

	return id.String()
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("Failed to marshal: " + err.Error())
	}

	return data
}
