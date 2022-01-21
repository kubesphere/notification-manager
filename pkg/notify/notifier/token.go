package notifier

import (
	"context"
	"sync"
	"time"

	"github.com/kubesphere/notification-manager/pkg/utils"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type token struct {
	accessToken   string
	accessTokenAt time.Time
	expires       time.Duration
	mutex         sync.Mutex
}

type AccessTokenService struct {
	mutex  sync.Mutex
	tokens map[string]token
}

var ats *AccessTokenService

func init() {
	ats = &AccessTokenService{
		tokens: make(map[string]token),
	}
}

func GetAccessTokenService() *AccessTokenService {
	return ats
}

func (ats *AccessTokenService) InvalidToken(ctx context.Context, key string, l log.Logger) {

	ats.mutex.Lock()
	defer ats.mutex.Unlock()

	ch := make(chan interface{})

	go func() {
		t, ok := ats.tokens[key]
		if ok {
			t.accessTokenAt = time.Time{}
		}
		ch <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		_ = level.Error(l).Log("msg", "invalid token timeout")
		return
	case <-ch:
		return
	}
}

func (ats *AccessTokenService) GetToken(ctx context.Context, key string, getToken func(ctx context.Context) (string, time.Duration, error)) (string, error) {

	ats.mutex.Lock()
	defer ats.mutex.Unlock()

	ch := make(chan interface{})

	go func() {
		t, ok := ats.tokens[key]
		if ok && time.Since(t.accessTokenAt) < t.expires {
			ch <- t.accessToken
			return
		}

		accessToken, expires, err := getToken(ctx)
		if err != nil {
			ch <- err
			return
		} else {
			ats.tokens[key] = token{
				accessToken:   accessToken,
				accessTokenAt: time.Now(),
				expires:       expires,
			}
			ch <- accessToken
			return
		}
	}()

	select {
	case <-ctx.Done():
		return "", utils.Error("get token timeout")
	case val := <-ch:
		switch val.(type) {
		case error:
			return "", val.(error)
		case string:
			return val.(string), nil
		default:
			return "", utils.Error("wrong token type")
		}
	}
}
