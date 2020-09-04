package dingtalk

import (
	"fmt"
	"sync"
	"time"
)

const (
	DefaultQPS = 20
)

var throttle *Throttle

type Throttle struct {
	qps          int
	maxWaitTime  time.Duration
	rateLimiters map[string]*rateLimiter
	mu           sync.Mutex
}

type rateLimiter struct {
	url   string
	queue []time.Time
	mu    sync.Mutex
}

func GetThrottle() *Throttle {
	if throttle == nil {
		throttle = &Throttle{
			qps:          DefaultQPS,
			rateLimiters: make(map[string]*rateLimiter),
		}
	}

	return throttle
}

func (t *Throttle) GetQPS() int {
	return t.qps
}

func (t *Throttle) SetQPS(qps int) {
	t.qps = qps
}

func (t *Throttle) GetMaxWaitTime() time.Duration {
	return t.maxWaitTime
}

func (t *Throttle) SetMaxWaitTime(wait time.Duration) {
	t.maxWaitTime = wait
}

func (t *Throttle) Add(url string) {

	t.mu.Lock()
	defer t.mu.Unlock()

	_, ok := t.rateLimiters[url]
	if !ok {
		t.rateLimiters[url] = &rateLimiter{
			url: url,
		}
	}
}

func (t *Throttle) Get(url string) *rateLimiter {
	t.mu.Lock()
	defer t.mu.Unlock()

	r, ok := t.rateLimiters[url]
	if !ok {
		return nil
	}

	return r
}

// This function calculates whether the webhook corresponding to the URL reaches the flow limit,
// if not, execute `send` to send the message, otherwise wait for the flow limit to be lifted, and send message.
// The max waiting time is set by `maxWaitTime`, if the actual waiting time is more than `maxWaitTime`, it will not wait.
//
// QPS defines the allowed message send to webhook per minute sent.
// The queue stores the time of the last (qps - 1) message sent.
//
// The logic of flow control is that the time for sending any consecutive qps‘s times messages cannot be greater than 1 minute.
//
// The queue stores the message sent time for the most recent qps-1 times,
// determine whether the flow limit is reached by comparing the current time and the first time in queue.
func (t *Throttle) Allow(url string, send func() error) (bool, error) {

	r := t.Get(url)
	if r == nil {
		t.Add(url)
		r = t.Get(url)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.queue) < t.qps {
		r.queue = append(r.queue, time.Now())
		return true, send()
	}

	now := time.Now()
	if now.Sub(r.queue[0]) >= time.Minute {
		r.queue = r.queue[1:]
		r.queue = append(r.queue, now)
		return true, send()
	} else {
		wait := time.Minute - now.Sub(r.queue[0])
		if wait <= t.maxWaitTime {
			time.Sleep(wait)
			r.queue = r.queue[1:]
			r.queue = append(r.queue, now)
			return true, send()
		} else {
			return false, fmt.Errorf("send to webhook %s failed because of rate limite, wait %d second, max wait %d second", url, wait, t.maxWaitTime)
		}
	}
}
