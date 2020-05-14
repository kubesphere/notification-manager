package wechat

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	commoncfg "github.com/prometheus/common/config"
	"net/http"
	"net/url"
	"time"
)

const (
	TokenChannleLen = 1000
	OpGet           = "get"
	OpInvalid       = "invalid"
	DefaultExpires  = time.Hour * 2
)

type token struct {
	AccessToken   string `json:"access_token,omitempty"`
	accessTokenAt time.Time
	Expires       time.Duration `json:"expires_in,omitempty"`
}

type operator struct {
	op     string
	resp   chan interface{}
	Config *config.WechatConfig
	ctx    context.Context
}

type AccessTokenService struct {
	tokens map[string]token
	ch     chan operator
	client *http.Client
}

var ats *AccessTokenService

func init() {
	ats = newAccessTokenService()
}

func newAccessTokenService() *AccessTokenService {
	c, err := commoncfg.NewClientFromConfig(commoncfg.HTTPClientConfig{}, "tokenService", false)
	if err != nil {
		_ = level.Error(log.NewNopLogger()).Log("msg", "WechatNotifier: create token service error", "error", err.Error())
		return nil
	}
	ats := &AccessTokenService{
		tokens: make(map[string]token),
		ch:     make(chan operator, TokenChannleLen),
		client: c,
	}
	go ats.run()
	return ats
}

func (ats *AccessTokenService) run() {

	for {
		select {
		case p, more := <-ats.ch:
			if !more {
				return
			}

			switch p.op {
			case OpGet:
				t, err := ats.getToken(p.Config, p.ctx)
				if err != nil {
					p.resp <- err
				} else {
					p.resp <- t
				}
			case OpInvalid:
				ats.invalidToken(p.Config)
			}
		}
	}
}

func (ats *AccessTokenService) getToken(c *config.WechatConfig, ctx context.Context) (string, error) {

	key := c.APIURL.String() + "|" + c.CorpID + "|" + string(c.APISecret) + "|" + c.AgentID

	t, ok := ats.tokens[key]
	if !ok {
		t = token{
			AccessToken: "",
			Expires:     DefaultExpires,
		}
	}

	if t.AccessToken == "" || time.Since(t.accessTokenAt) > t.Expires {
		parameters := url.Values{}
		parameters.Add("corpsecret", string(c.APISecret))
		parameters.Add("corpid", c.CorpID)

		u := c.APIURL.Copy()
		u.Path += "gettoken"
		u.RawQuery = parameters.Encode()

		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return "", err
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := ats.client.Do(req.WithContext(ctx))
		if err != nil {
			return "", err
		}
		defer notify.Drain(resp)

		var wechatToken token
		if err := jsoniter.NewDecoder(resp.Body).Decode(&wechatToken); err != nil {
			return "", err
		}

		if wechatToken.AccessToken == "" {
			return "", fmt.Errorf("invalid APISecret for APIURL: %s, CorpID: %s, APISecret: %s,, AgentID: %s",
				c.APIURL, c.CorpID, c.APISecret, c.AgentID)
		}

		// Cache accessToken
		t.AccessToken = wechatToken.AccessToken
		t.accessTokenAt = time.Now()
		t.Expires = t.Expires * time.Second
	}

	return t.AccessToken, nil
}

func (ats *AccessTokenService) invalidToken(c *config.WechatConfig) {

	key := c.APIURL.String() + "|" + c.CorpID + "|" + string(c.APISecret) + "|" + c.AgentID

	t, ok := ats.tokens[key]
	if ok {
		t.AccessToken = ""
	}

	return
}

func (ats *AccessTokenService) get(c *config.WechatConfig, ctx context.Context, resp chan interface{}) {

	p := operator{
		op:     OpGet,
		Config: c,
		ctx:    ctx,
		resp:   resp,
	}

	ats.ch <- p
}

func (ats *AccessTokenService) invalid(c *config.WechatConfig) {
	p := operator{
		op:     OpInvalid,
		Config: c,
	}

	ats.ch <- p
}
