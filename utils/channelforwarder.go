package utils

import (
	"sync"
)

type ChannelForwarder[T any] struct {
	original chan T

	channels []chan T
	isClosed bool

	buffer      int
	savedValues []T
	lock        sync.Mutex
}

func ForwardChannel[T any](orig chan T, buffer int, saveValues bool) *ChannelForwarder[T] {
	cf := &ChannelForwarder[T]{
		original: orig,
		isClosed: false,
		buffer:   buffer,
	}

	go func() {
		for {
			event, ok := <-orig

			cf.lock.Lock()
			if !ok {
				for _, cha := range cf.channels {
					close(cha)
				}
				cf.channels = nil
				cf.isClosed = true

				cf.lock.Unlock()
				return
			}

			if saveValues {
				cf.savedValues = append(cf.savedValues, event)
			}

			for _, forwardChannel := range cf.channels {
				forwardChannel <- event
			}
			cf.lock.Unlock()
		}
	}()

	return cf
}

func (c *ChannelForwarder[T]) Remove(recv <-chan T) {
	select {
	case <-recv:
	default:
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	if len(c.channels) == 0 {
		return
	}

	var i int
	for j, cha := range c.channels {
		if recv == cha {
			i = j
			break
		}
	}

	close(c.channels[i])
	c.channels[i] = c.channels[len(c.channels)-1]
	c.channels = c.channels[:len(c.channels)-1]
}

func (c *ChannelForwarder[T]) Get() <-chan T {
	newChan := make(chan T, c.buffer)

	c.lock.Lock()
	if c.isClosed {
		return nil
	}

	c.channels = append(c.channels, newChan)

	if len(c.savedValues) > 0 {
		go func() {
			defer c.lock.Unlock()

			for _, val := range c.savedValues {
				newChan <- val
			}
		}()
	} else {
		c.lock.Unlock()
	}

	return newChan
}

func (c *ChannelForwarder[T]) Send(val T) {
	c.original <- val
}

func (c *ChannelForwarder[T]) Close() {
	if !c.isClosed {
		c.isClosed = true
		close(c.original)
	}
}
