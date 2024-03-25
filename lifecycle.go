package Artifex

import (
	"sync"
)

// Lifecycle define a management mechanism when obj creation and obj end.
type Lifecycle struct {
	spawnHandlers []func() error
	exitHandlers  []func() error

	exitNotify chan struct{}
	onceClose  sync.Once
}

func (life *Lifecycle) AddSpawnHandler(spawnHandlers ...func() error) {
	life.spawnHandlers = append(life.spawnHandlers, spawnHandlers...)
}

func (life *Lifecycle) AddExitHandler(exitHandlers ...func() error) {
	life.exitHandlers = append(life.exitHandlers, exitHandlers...)
}

func (life *Lifecycle) NotifyExit() {
	if life.exitNotify == nil {
		return
	}
	life.onceClose.Do(func() {
		close(life.exitNotify)
	})
}

func (life *Lifecycle) Execute() error {
	err := life.spawn()
	if err != nil {
		return err
	}

	if len(life.exitHandlers) == 0 {
		return nil
	}

	life.exitNotify = make(chan struct{})
	life.onceClose = sync.Once{}

	go func() {
		<-life.exitNotify
		life.exit()
	}()

	return nil
}

func (life *Lifecycle) spawn() error {
	if len(life.spawnHandlers) == 0 {
		return nil
	}
	for _, enter := range life.spawnHandlers {
		err := enter()
		if err != nil {
			life.exit()
			return err
		}
	}
	return nil
}

func (life *Lifecycle) exit() {
	if len(life.exitHandlers) == 0 {
		return
	}
	for _, action := range life.exitHandlers {
		action()
	}
	return
}
