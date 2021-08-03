package pushover

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Pushover API limitations.
const (
	// MessageMaxLength is the maximum length of the message, currently limited to 1024 4-byte UTF-8 characters
	MessageMaxLength = 1024
	// TitleMaxLength is the maximum length of the title, up to 250 characters.
	TitleMaxLength = 250
)

var (
	tokenRegex  = regexp.MustCompile(`^[A-Za-z0-9]{30}$`)
	deviceRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,25}$`)
	sounds      = map[string]bool{
		"pushover":     true,
		"bike":         true,
		"bugle":        true,
		"cashregister": true,
		"classical":    true,
		"cosmic":       true,
		"falling":      true,
		"gamelan":      true,
		"incoming":     true,
		"intermission": true,
		"magic":        true,
		"mechanical":   true,
		"pianobar":     true,
		"siren":        true,
		"spacealarm":   true,
		"tugboat":      true,
		"alien":        true,
		"climb":        true,
		"persistent":   true,
		"echo":         true,
		"updown":       true,
		"vibrate":      true,
		"none":         true,
	}
)

// Pushover message struct
type pushoverMessage struct {
	// required fields
	// Token is a Pushover application API token, required.
	Token string `json:"token"`
	// UserKey is recipient's Pushover User Key, required.
	UserKey string `json:"user"`
	// Message is your text message, required.
	Message string `json:"message"`

	// common optional fields
	// Device specifies a set of user's devices to send the message; all would be sent if empty
	Device string `json:"device,omitempty"`
	// Title is the message's title, otherwise application's name is used.
	Title string `json:"title,omitempty"`
	// Sound is the name of one of the sounds supported by device clients.
	Sound string `json:"sound,omitempty"`
}

func newPushoverMessage(token string, userKey string, message string) pushoverMessage {
	return pushoverMessage{Token: token, UserKey: userKey, Message: message, Sound: "pushover"}
}

func newPushoverMessageExtend(token, userKey, message, title, sound string, devices []string) pushoverMessage {
	return pushoverMessage{Token: token, UserKey: userKey, Message: message, Sound: sound, Title: title, Device: strings.Join(devices, ",")}
}

func (p *pushoverMessage) validate() (err error, warns []string) {
	// Validate the application API token.
	if !tokenRegex.MatchString(p.Token) {
		err = fmt.Errorf("invalid Pushover application API token: %s", p.Token)
		return
	}

	// Validate the User Key.
	if !tokenRegex.MatchString(p.UserKey) {
		err = fmt.Errorf("invalid Pushover User Key: %s", p.UserKey)
		return
	}

	// Validate the length of the message.
	msgLen := utf8.RuneCountInString(p.Message)
	if msgLen <= 0 {
		err = fmt.Errorf("message should not be empty")
		return
	}
	if msgLen > MessageMaxLength {
		warns = append(warns, fmt.Sprintf("the part of this message that exceeds %d runes is ignored", MessageMaxLength))
	}

	// Validate devices if configured.
	if len(p.Device) > 0 {
		for _, d := range strings.Split(p.Device, ",") {
			if !deviceRegex.MatchString(d) {
				err = fmt.Errorf("invalid device: %s, check if you splitted your devices with comma", d)
				return
			}
		}
	}

	// Validate the length of title.
	if l := utf8.RuneCountInString(p.Title); l > TitleMaxLength {
		err = fmt.Errorf("invalid title length, should be < %d, actually %d", TitleMaxLength, l)
		return
	}

	// Validate sound.
	if len(p.Sound) > 0 && !sounds[p.Sound] {
		warns = append(warns, fmt.Sprintf("not supported sound: %s, replaced with default sound", p.Sound))
		p.Sound = "pushover"
	}

	return
}
