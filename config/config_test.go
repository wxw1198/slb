package config

import (
	"fmt"
	"testing"
)

func TestGetConfig(t *testing.T) {
	cf := GetConfig()
	fmt.Printf("%+v", cf)
	if cf != nil {
		fmt.Println("true")
	}
}
