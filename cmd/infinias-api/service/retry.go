package service

import (
	"log"
	"math/rand"
	"time"
)

type RetryStrategy struct {
	Initial     time.Duration
	MaxRetries  uint
	MaxDuration time.Duration
	MaxJitter   time.Duration
}

func (s *RetryStrategy) Retry(f func() error) error {
	tries := 0
	backoff := s.Initial
	for {
		err := f()
		if err == nil {
			return nil
		}

		tries += 1
		if tries == int(s.MaxRetries) {
			return err
		}

		if backoff > s.MaxDuration {
			backoff = s.MaxDuration
		}

		dur := backoff + time.Duration(rand.Int63n(int64(s.MaxJitter)))
		log.Printf("service failed unexpectedly (retry in %v): %v\n", dur, err)

		time.Sleep(dur)
		backoff *= 2
	}
}

var DefaultRetryStrategy = &RetryStrategy{
	Initial:     time.Second,
	MaxRetries:  9999,
	MaxDuration: 60 * time.Second,
	MaxJitter:   time.Second,
}
