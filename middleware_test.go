//go:build longtime
// +build longtime

package Artifex

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestUse_Retry(t *testing.T) {
	use := Use{}

	start := time.Now()

	msg := Message{
		Subject: "Rtry",
	}
	retryMaxSecond := 20
	task := func(message *Message, dependency any) error {
		diff := time.Now().Sub(start)
		fmt.Println(message.Subject, diff)
		if diff > 5*time.Second {
			return nil
		}
		return errors.New("timeout")
	}
	err := LinkMiddlewares(task, use.Retry(retryMaxSecond))(&msg, nil)
	if err != nil {
		t.Errorf("unexpected output: %v", err)
	}
}
