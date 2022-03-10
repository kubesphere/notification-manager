package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math"
	"math/rand"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/async"
	"github.com/kubesphere/notification-manager/pkg/constants"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/internal"
	"github.com/kubesphere/notification-manager/pkg/internal/email"
	"github.com/kubesphere/notification-manager/pkg/notify/notifier"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	MaxEmailReceivers       = math.MaxInt32
	DefaultSendTimeout      = time.Second * 3
	DefaultHTMLTemplate     = `{{ template "nm.default.html" . }}`
	DefaultTextTemplate     = `{{ template "nm.default.text" . }}`
	DefaultTSubjectTemplate = `{{ template "nm.default.subject" . }}`
)

type Notifier struct {
	notifierCtl *controller.Controller
	receiver    *email.Receiver
	tmpl        *template.Template
	timeout     time.Duration
	logger      log.Logger
	// Email delivery type, single or bulk.
	delivery string
	// The maximum size of receivers in one email.
	maxEmailReceivers int
}

func NewEmailNotifier(logger log.Logger, receiver internal.Receiver, notifierCtl *controller.Controller) (notifier.Notifier, error) {

	n := &Notifier{
		notifierCtl:       notifierCtl,
		logger:            logger,
		timeout:           DefaultSendTimeout,
		maxEmailReceivers: MaxEmailReceivers,
	}

	opts := notifierCtl.ReceiverOpts
	tmplType := constants.HTML
	subjectTmplName := DefaultTSubjectTemplate
	tmplName := ""
	if opts != nil && opts.Global != nil && !utils.StringIsNil(opts.Global.Template) {
		tmplName = opts.Global.Template
	}

	if opts != nil && opts.Email != nil {
		if opts.Email.NotificationTimeout != nil {
			n.timeout = time.Second * time.Duration(*opts.Email.NotificationTimeout)
		}

		if opts.Email.MaxEmailReceivers > 0 {
			n.maxEmailReceivers = opts.Email.MaxEmailReceivers
		}

		if !utils.StringIsNil(opts.Email.Template) {
			tmplName = opts.Email.Template
		}

		if !utils.StringIsNil(opts.Email.TmplType) {
			tmplType = opts.Email.TmplType
		}

		if !utils.StringIsNil(opts.Email.SubjectTemplate) {
			subjectTmplName = opts.Email.SubjectTemplate
		}
	}

	n.receiver = receiver.(*email.Receiver)
	if n.receiver.Config == nil {
		_ = level.Warn(logger).Log("msg", "EmailNotifier: ignore receiver because of empty config")
		return nil, utils.Error("ignore receiver because of empty config")
	}

	if utils.StringIsNil(n.receiver.TmplType) {
		n.receiver.TmplType = tmplType
	}

	if utils.StringIsNil(n.receiver.TmplName) {
		if tmplName != "" {
			n.receiver.TmplName = tmplName
		} else {
			if n.receiver.TmplType == constants.HTML {
				n.receiver.TmplName = DefaultHTMLTemplate
			} else if n.receiver.TmplType == constants.Text {
				n.receiver.TmplName = DefaultTextTemplate
			}
		}
	}

	if utils.StringIsNil(n.receiver.TitleTmplName) {
		n.receiver.TitleTmplName = subjectTmplName
	}

	var err error
	n.tmpl, err = notifierCtl.GetReceiverTmpl(n.receiver.TmplText)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "EmailNotifier: create receiver template error", "error", err.Error())
		return nil, err
	}

	return n, nil
}

func (n *Notifier) Notify(ctx context.Context, data *template.Data) error {

	sendEmail := func(to, subject, body string) error {

		start := time.Now()
		defer func() {
			_ = level.Debug(n.logger).Log("msg", "EmailNotifier: send message", "used", time.Since(start).String())
		}()

		err := n.send(ctx, to, subject, body)
		if err != nil {
			_ = level.Error(n.logger).Log("msg", "EmailNotifier: notify error", "from", n.receiver.From, "to", n.receiver.To, "error", err.Error())
			return err
		}

		_ = level.Debug(n.logger).Log("msg", "EmailNotifier: send message", "from", n.receiver.From, "to", utils.ArrayToString(n.receiver.To, ","))
		return nil
	}

	subject, err := n.tmpl.Text(n.receiver.TitleTmplName, data)
	if err != nil {
		_ = level.Error(n.logger).Log("msg", "EmailNotifier: create email subject error", err.Error())
		return err
	}

	body := ""
	if n.receiver.TmplType != constants.Text {
		body, err = n.tmpl.Text(n.receiver.TmplName, data)
	} else if n.receiver.TmplType != constants.HTML {
		body, err = n.tmpl.Html(n.receiver.TmplName, data)
	} else {
		_ = level.Error(n.logger).Log("msg", "EmailNotifier: unknown message type", "type", n.receiver.TmplType)
		return utils.Errorf("Unknown message type, %s", n.receiver.TmplType)
	}

	group := async.NewGroup(ctx)
	for _, t := range n.receiver.To {
		to := t
		group.Add(func(stopCh chan interface{}) {
			stopCh <- sendEmail(to, subject, body)
		})
	}

	return group.Wait()
}

func (n *Notifier) send(ctx context.Context, to, subject, body string) error {

	addr := fmt.Sprintf("%s:%d", n.receiver.SmartHost.Host, n.receiver.SmartHost.Port)
	var err error
	var conn net.Conn
	if n.receiver.SmartHost.Port == 465 {
		tlsConfig, err := n.newTLSConfig()
		if err != nil {
			return err
		}
		conn, err = tls.Dial("tcp", addr, tlsConfig)
	} else {
		d := net.Dialer{}
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, n.receiver.SmartHost.Host)
	if err != nil {
		return err
	}
	defer func() {
		if err := c.Quit(); err != nil {
			_ = level.Warn(n.logger).Log("msg", "failed to close SMTP connection", "err", err)
		}
	}()

	if !utils.StringIsNil(n.receiver.Hello) {
		if err = c.Hello(n.receiver.Hello); err != nil {
			return err
		}
	}

	// Global Config guarantees RequireTLS is not nil.
	if n.receiver.RequireTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return utils.Errorf("'require_tls' is true (default) but %q does not advertise the STARTTLS extension", n.receiver.SmartHost)
		}

		tlsConf, err := n.newTLSConfig()
		if err != nil {
			return err
		}

		if err := c.StartTLS(tlsConf); err != nil {
			return err
		}
	}

	if ok, mech := c.Extension("AUTH"); ok {
		auth, err := n.auth(mech)
		if err != nil {
			return err
		}
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}

	addrs, err := mail.ParseAddressList(n.receiver.From)
	if err != nil {
		return err
	}
	if len(addrs) != 1 {
		return utils.Errorf("must be exactly one 'from' address (got: %d)", len(addrs))
	}
	if err = c.Mail(addrs[0].Address); err != nil {
		return err
	}

	addrs, err = mail.ParseAddressList(to)
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		if err = c.Rcpt(addr.Address); err != nil {
			return err
		}
	}

	// Send the email headers and body.
	message, err := c.Data()
	if err != nil {
		return err
	}

	multipartBuffer := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBuffer)
	defer func() {
		_ = multipartWriter.Close()
		_ = message.Close()
	}()

	buffer := &bytes.Buffer{}
	_, _ = fmt.Fprintf(buffer, "From: %s\r\n", mime.QEncoding.Encode("utf-8", n.receiver.From))
	_, _ = fmt.Fprintf(buffer, "To: %s\r\n", mime.QEncoding.Encode("utf-8", to))
	_, _ = fmt.Fprintf(buffer, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	_, _ = fmt.Fprintf(buffer, "Message-Id: %s\r\n", fmt.Sprintf("<%d.%d@%s>", time.Now().UnixNano(), rand.Uint64(), n.receiver.SmartHost.Host))
	_, _ = fmt.Fprintf(buffer, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	_, _ = fmt.Fprintf(buffer, "Content-Type: multipart/alternative;  boundary=%s\r\n", multipartWriter.Boundary())
	_, _ = fmt.Fprintf(buffer, "MIME-Version: 1.0\r\n\r\n")

	_, err = message.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	var w io.Writer
	if n.receiver.TmplType == constants.Text {
		w, err = multipartWriter.CreatePart(textproto.MIMEHeader{
			"Content-Transfer-Encoding": {"quoted-printable"},
			"Content-Type":              {"text/plain; charset=UTF-8"},
		})
	} else {
		w, err = multipartWriter.CreatePart(textproto.MIMEHeader{
			"Content-Transfer-Encoding": {"quoted-printable"},
			"Content-Type":              {"text/html; charset=UTF-8"},
		})
	}
	if err != nil {
		return err
	}

	qw := quotedprintable.NewWriter(w)
	if _, err = qw.Write([]byte(body)); err != nil {
		return err
	}
	_ = qw.Close()

	_, err = message.Write(multipartBuffer.Bytes())
	return err
}

func (n *Notifier) newTLSConfig() (*tls.Config, error) {

	if n.receiver.TLS == nil {
		return &tls.Config{InsecureSkipVerify: true}, nil
	}

	t := n.receiver.TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: t.InsecureSkipVerify,
		ServerName:         t.ServerName,
	}

	if utils.StringIsNil(tlsConfig.ServerName) {
		tlsConfig.ServerName = n.receiver.SmartHost.Host
	}

	// If a CA cert is provided then let's read it in, so we can validate the
	// scrape target's certificate properly.
	if t.RootCA != nil {
		if ca, err := n.notifierCtl.GetCredential(t.RootCA); err != nil {
			return nil, err
		} else {
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM([]byte(ca)) {
				return nil, err
			}
			tlsConfig.RootCAs = caCertPool
		}
	}

	// If a client cert & key is provided then configure TLS config accordingly.
	if t.ClientCertificate != nil {
		if t.Cert != nil && t.Key == nil {
			return nil, utils.Error("Client cert file specified without client key file")
		} else if t.Cert == nil && t.Key != nil {
			return nil, utils.Error("Client key file specified without client cert file")
		} else if t.Cert != nil && t.Key != nil {
			key, err := n.notifierCtl.GetCredential(t.Key)
			if err != nil {
				return nil, err
			}

			cert, err := n.notifierCtl.GetCredential(t.Cert)
			if err != nil {
				return nil, err
			}

			tlsCert, err := tls.X509KeyPair([]byte(cert), []byte(key))
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{tlsCert}
		}
	}

	return tlsConfig, nil
}

func (n *Notifier) auth(mechs string) (smtp.Auth, error) {
	username := n.receiver.AuthUsername
	if username == "" {
		return nil, nil
	}

	for _, mech := range strings.Split(mechs, " ") {
		switch mech {
		case "CRAM-MD5":
			secret, err := n.notifierCtl.GetCredential(n.receiver.AuthSecret)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "EmailNotifier: get authSecret error", "error", err.Error())
				continue
			}
			return smtp.CRAMMD5Auth(username, secret), nil

		case "PLAIN":
			password, err := n.notifierCtl.GetCredential(n.receiver.AuthPassword)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "EmailNotifier: get authPassword error", "error", err.Error())
				continue
			}
			identity := n.receiver.AuthIdentify

			return smtp.PlainAuth(identity, username, password, n.receiver.SmartHost.Host), nil
		case "LOGIN":
			password, err := n.notifierCtl.GetCredential(n.receiver.AuthPassword)
			if err != nil {
				_ = level.Error(n.logger).Log("msg", "EmailNotifier: get authPassword error", "error", err.Error())
				continue
			}
			return LoginAuth(username, password), nil
		}
	}

	return nil, utils.Errorf("unknown auth mechanism: %s", mechs)
}

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch strings.ToLower(string(fromServer)) {
		case "username:":
			return []byte(a.username), nil
		case "password:":
			return []byte(a.password), nil
		default:
			return nil, utils.Error("unexpected server challenge")
		}
	}
	return nil, nil
}
