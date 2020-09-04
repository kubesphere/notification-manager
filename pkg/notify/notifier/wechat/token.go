package wechat

import (
	"context"
	"fmt"
	json "github.com/json-iterator/go"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"net/http"
	"time"
)

const (
	TokenChannelLen = 1000
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
	op        string
	resp      chan interface{}
	apiURL    string
	corpID    string
	apiSecret string
	agentID   string
	ctx       context.Context
}

type AccessTokenService struct {
	tokens map[string]token
	ch     chan operator
}

var ats *AccessTokenService

func init() {
	ats = newAccessTokenService()
}

func newAccessTokenService() *AccessTokenService {

	ats := &AccessTokenService{
		tokens: make(map[string]token),
		ch:     make(chan operator, TokenChannelLen),
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
				t, err := ats.getToken(p.apiURL, p.corpID, p.apiSecret, p.agentID, p.ctx)
				if err != nil {
					p.resp <- err
				} else {
					p.resp <- t
				}
			case OpInvalid:
				ats.invalidToken(p.apiURL, p.corpID, p.apiSecret, p.agentID)
			}
		}
	}
}

func (ats *AccessTokenService) getToken(apiURL, corpID, apiSecret, agentID string, ctx context.Context) (string, error) {

	key := apiURL + "|" + corpID + "|" + apiSecret + "|" + agentID

	t, ok := ats.tokens[key]
	if !ok {
		t = token{
			AccessToken: "",
			Expires:     DefaultExpires,
		}
	}

	if t.AccessToken == "" || time.Since(t.accessTokenAt) > t.Expires {

		u, err := notifier.UrlWithPath(apiURL, "gettoken")
		if err != nil {
			return "", err
		}

		parameters := make(map[string]string)
		parameters["corpsecret"] = apiSecret
		parameters["corpid"] = corpID
		u, err = notifier.UrlWithParameters(u, parameters)
		if err != nil {
			return "", err
		}

		request, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(request.WithContext(ctx))
		if err != nil {
			return "", err
		}

		var wechatToken token
		if err := json.NewDecoder(resp.Body).Decode(&wechatToken); err != nil {
			return "", err
		}

		if wechatToken.AccessToken == "" {
			return "", fmt.Errorf("invalid APISecret for APIURL: %s, CorpID: %s, APISecret: %s,, AgentID: %s",
				apiURL, corpID, apiSecret, agentID)
		}

		// Cache accessToken
		t.AccessToken = wechatToken.AccessToken
		t.accessTokenAt = time.Now()
		t.Expires = t.Expires * time.Second
	}

	return t.AccessToken, nil
}

func (ats *AccessTokenService) invalidToken(apiURL, corpID, apiSecret, agentID string) {

	key := apiURL + "|" + corpID + "|" + apiSecret + "|" + agentID

	t, ok := ats.tokens[key]
	if ok {
		t.AccessToken = ""
	}

	return
}

func (ats *AccessTokenService) get(apiURL, corpID, apiSecret, agentID string, ctx context.Context, resp chan interface{}) {

	p := operator{
		op:        OpGet,
		apiURL:    apiURL,
		corpID:    corpID,
		apiSecret: apiSecret,
		agentID:   agentID,
		ctx:       ctx,
		resp:      resp,
	}

	ats.ch <- p
}

func (ats *AccessTokenService) invalid(apiURL, corpID, apiSecret, agentID string) {
	p := operator{
		op:        OpInvalid,
		apiURL:    apiURL,
		corpID:    corpID,
		apiSecret: apiSecret,
		agentID:   agentID,
	}

	ats.ch <- p
}
