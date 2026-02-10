package game

import "github.com/google/uuid"

func GenID() string {
	id, err := uuid.NewV7()
    if err != nil {
        panic("Failed to generate UUID: " + err.Error())
    }
    
    return id.String()
}