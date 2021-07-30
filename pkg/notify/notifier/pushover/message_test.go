package pushover

import (
	"testing"
)

func Test_pushoverMessage_validate(t *testing.T) {
	p := newPushoverMessage("avrgpab2qgb4rzhm3un826o7600000", "ur99hih8czsgv4xaqsetseefr00000", "hello")
	err, warns := p.validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, warn := range warns {
		t.Log(warn)
	}
}
