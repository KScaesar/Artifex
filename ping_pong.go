package Artifex

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/exp/constraints"
)

type PingPong[S constraints.Ordered, rM, sM any] struct {
	Enable bool

	// When SendPingWaitPong sends a ping message and waits for a corresponding pong message.
	// SendPeriod = WaitSecond / 2
	//
	// When WaitPingSendPong waits for a ping message and response a corresponding pong message.
	// SendPeriod = WaitSecond
	WaitSecond int // Must,  when enable pingpong

	// Example:
	// 	custom protocol-> "ping" or "pong"
	//	ws protocol -> PongMessage = 10, PingMessage = 9
	WaitSubject S                                    // Must,  when enable pingpong
	SendFunc    func(sess *Session[S, rM, sM]) error // Must,  when enable pingpong

	IsSendPingWaitPong bool           // Option,  when enable pingpong
	WaitFunc           HandleFunc[rM] // Option,  when enable pingpong

}

func (pingpong PingPong[S, rM, sM]) Run(sess *Session[S, rM, sM]) error {
	if !pingpong.Enable {
		return nil
	}

	if pingpong.WaitFunc == nil {
		pingpong.WaitFunc = func(_ rM, _ *RouteParam) error { return nil }
	}

	mux := sess.recvMux

	waitNotify := make(chan error, 1)
	mux.Handler(pingpong.WaitSubject, func(message rM, route *RouteParam) error {
		waitNotify <- pingpong.WaitFunc(message, route)
		return nil
	})

	sendFunc := func() error { return pingpong.SendFunc(sess) }

	if pingpong.IsSendPingWaitPong {
		return SendPingWaitPong(sendFunc, waitNotify, sess.IsStop, pingpong.WaitSecond)
	}
	return WaitPingSendPong(waitNotify, sendFunc, sess.IsStop, pingpong.WaitSecond)
}

func WaitPingSendPong(waitPing <-chan error, sendPong func() error, isStop func() bool, pingWaitSecond int) error {
	pingWaitTime := time.Duration(pingWaitSecond) * time.Second

	timer := time.NewTimer(pingWaitTime)
	defer timer.Stop()

	for !isStop() {
		select {
		case <-timer.C:
			return errors.New("wait ping timeout")

		case err := <-waitPing:
			if err != nil {
				return fmt.Errorf("wait ping: %v", err)
			}

			err = sendPong()
			if err != nil {
				return fmt.Errorf("send pong: %v", err)
			}

			ok := timer.Reset(pingWaitTime)
			if !ok {
				timer = time.NewTimer(pingWaitTime)
			}
		}
	}

	return nil
}

func SendPingWaitPong(ping func() error, pong <-chan error, isStop func() bool, pongWaitSecond int) error {
	pongWaitTime := time.Duration(pongWaitSecond) * time.Second
	pingPeriod := pongWaitTime / 2

	done := make(chan struct{})
	defer close(done)

	result := make(chan error, 2)

	sendPing := func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for !isStop() {
			select {
			case <-ticker.C:
				err := ping()
				if err != nil {
					result <- fmt.Errorf("Send ping: %v", err)
					return
				}

			case <-done:
				return
			}
		}
		result <- nil
	}

	waitPong := func() {
		timer := time.NewTimer(pongWaitTime)
		defer timer.Stop()

		for !isStop() {
			select {
			case <-timer.C:
				result <- errors.New("wait pong timeout")
				return

			case err := <-pong:
				if err != nil {
					result <- fmt.Errorf("handle pong: %v", err)
					return
				}

				ok := timer.Reset(pongWaitTime)
				if !ok {
					timer = time.NewTimer(pongWaitTime)
				}

			case <-done:
				return
			}
		}
		result <- nil
	}

	go sendPing()
	go waitPong()
	return <-result
}
