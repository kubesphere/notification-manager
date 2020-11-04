package dingtalk

import (
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"sync"
	"time"
)

var throttle *Throttle

type Throttle struct {
	rateLimiters map[string]*rateLimiter
	mutex        sync.Mutex
}

type rateLimiter struct {
	key         string
	threshold   int
	unitTime    time.Duration
	maxWaitTime time.Duration
	queue       []time.Time
	mutex       sync.Mutex
}

func init() {
	throttle = &Throttle{
		rateLimiters: make(map[string]*rateLimiter),
	}
}

func GetThrottle() *Throttle {
	return throttle
}

// Add, if exist, update
func (t *Throttle) Add(key string, threshold int, unitTime time.Duration, maxWaitTime time.Duration) {

	t.mutex.Lock()
	defer t.mutex.Unlock()

	r, ok := t.rateLimiters[key]
	if !ok {
		t.rateLimiters[key] = &rateLimiter{
			key:         key,
			threshold:   threshold,
			unitTime:    unitTime,
			maxWaitTime: maxWaitTime,
		}

		return
	}

	t.updateRateLimiter(r, threshold, unitTime, maxWaitTime)
}

func (t *Throttle) updateRateLimiter(r *rateLimiter, threshold int, unitTime time.Duration, maxWaitTime time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.threshold = threshold
	r.unitTime = unitTime
	r.maxWaitTime = maxWaitTime
}

// Add, if exist, do nothing
func (t *Throttle) TryAdd(key string, threshold int, unitTime time.Duration, maxWaitTime time.Duration) {

	t.mutex.Lock()
	defer t.mutex.Unlock()

	_, ok := t.rateLimiters[key]
	if !ok {
		t.rateLimiters[key] = &rateLimiter{
			key:         key,
			threshold:   threshold,
			unitTime:    unitTime,
			maxWaitTime: maxWaitTime,
		}
	}
}

func (t *Throttle) Get(url string) *rateLimiter {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	r, ok := t.rateLimiters[url]
	if !ok {
		return nil
	}

	return r
}

// This function calculates whether the api calls reaches the flow limit,
// if not, return true, otherwise wait for the flow limit to be lifted, and return true.
// The max waiting time is set by `maxWaitTime`, if the actual waiting time is more than `maxWaitTime`, it will not wait, and return false.
//
// The `threshold` defines the allowed calls per `unitTime`.
// The queue stores the time of the last (threshold - 1) call.
//
// The logic of flow control is that the time of any `threshold` consecutive calls cannot be greater than `unitTime`.
//
func (t *Throttle) Allow(key string, logger log.Logger) bool {

	r := t.Get(key)
	if r == nil {
		_ = level.Error(logger).Log("msg", "Throttle: key is not exist", "key", key)
		return false
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	l := len(r.queue)
	if l < r.threshold {
		r.queue = append(r.queue, time.Now())
		return true
	}

	// If the threshold has changed, truncate it.
	r.queue = r.queue[l-r.threshold:]

	now := time.Now()
	if now.Sub(r.queue[0]) >= r.unitTime {
		r.queue = r.queue[1:]
		r.queue = append(r.queue, now)
		return true
	} else {
		wait := r.unitTime - now.Sub(r.queue[0])
		if wait <= r.maxWaitTime {
			_ = level.Debug(logger).Log("msg", "Throttle: wait start", "key", key, "time", wait.String())
			time.Sleep(wait)
			_ = level.Debug(logger).Log("msg", "Throttle: wait end", "key", key, "time", wait.String())
			r.queue = r.queue[1:]
			r.queue = append(r.queue, time.Now())
			return true
		} else {
			_ = level.Error(logger).Log("msg", "Throttle: drop", "key", key, "time", wait.String(), "max wait time", r.maxWaitTime)
			return false
		}
	}
}
