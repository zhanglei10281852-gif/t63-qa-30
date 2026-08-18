package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Check func(context.Context) error
type Result struct {
	Name     string        `json:"name"`
	Healthy  bool          `json:"healthy"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}
type Checker struct {
	Timeout time.Duration
	Checks  map[string]Check
}

func (c Checker) Run(ctx context.Context) ([]Result, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	results := make([]Result, 0, len(c.Checks))
	channel := make(chan Result, len(c.Checks))
	var group sync.WaitGroup
	for name, check := range c.Checks {
		name, check := name, check
		group.Add(1)
		go func() {
			defer group.Done()
			started := time.Now()
			result := Result{Name: name}
			if check == nil {
				result.Error = "check is nil"
			} else if err := check(ctx); err != nil {
				result.Error = err.Error()
			} else {
				result.Healthy = true
			}
			result.Duration = time.Since(started)
			channel <- result
		}()
	}
	group.Wait()
	close(channel)
	healthy := true
	for result := range channel {
		results = append(results, result)
		if !result.Healthy {
			healthy = false
		}
	}
	if !healthy {
		return results, fmt.Errorf("one or more dependencies are not ready")
	}
	return results, nil
}
