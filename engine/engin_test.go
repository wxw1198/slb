package engine

import (
	"fmt"
	"slb/config"
	"testing"
)

func TestNewSlb(t *testing.T) {

	cf := config.GetConfig()
	if cf == nil {
		fmt.Println("test ")
	}
	slb := NewSlb(cf)

	slb.Run()
}
