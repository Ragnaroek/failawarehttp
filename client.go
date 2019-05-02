package http

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var random *rand.Rand
var log *logrus.Logger

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))

	logger := logrus.StandardLogger()
	logrus.SetLevel(logLevel())

	log = logger
}

func logLevel() logrus.Level {
	logEnv := os.Getenv("LOG_LEVEL")
	switch logEnv {
	case "": //not set
		return logrus.ErrorLevel
	case "panic":
		return logrus.PanicLevel
	case "fatal":
		return logrus.FatalLevel
	case "error":
		return logrus.ErrorLevel
	case "warn":
		return logrus.WarnLevel
	case "info":
		return logrus.InfoLevel
	case "debug":
		return logrus.DebugLevel
	case "trace":
		return logrus.TraceLevel
	}

	panic(fmt.Sprintf("LOG_LEVEL %s is not known", logEnv))
}

//FailAwareHTTPClient is the extendes HTTP client. It provides the same methods as the
//http.Client.
type FailAwareHTTPClient struct {
	httpClient *http.Client
	options    FailAwareHTTPOptions
}

//FailAwareHTTPOptions are the options for the FFailAwareHttp client.
//See NewClient(options) and ddefaultOptions.
type FailAwareHTTPOptions struct {
	MaxRetries         int
	Timeout            time.Duration
	BackOffDelayFactor time.Duration
	KeepLog            bool
}

var defaultOptions = NewDefaultOptions()
var nullOptions = FailAwareHTTPOptions{}

//NewDefaultOptions creates new default options for the client.
func NewDefaultOptions() FailAwareHTTPOptions {
	return FailAwareHTTPOptions{
		MaxRetries:         3,
		Timeout:            1 * time.Second,
		BackOffDelayFactor: 1 * time.Second,
		KeepLog:            false,
	}
}

//NewDefaultClient creates a FailAwareHTTP client with defaultOptions.
func NewDefaultClient() *FailAwareHTTPClient {
	return NewClient(defaultOptions)
}

//NewClient creates a new FFailAwareHTTP client.
func NewClient(options FailAwareHTTPOptions) *FailAwareHTTPClient {

	var timeout time.Duration
	if options.Timeout == nullOptions.Timeout {
		timeout = defaultOptions.Timeout
	} else {
		timeout = options.Timeout
	}

	var maxRetries int
	if options.MaxRetries == nullOptions.MaxRetries {
		maxRetries = defaultOptions.MaxRetries
	} else {
		maxRetries = options.MaxRetries
	}

	var backOffDelay time.Duration
	if options.BackOffDelayFactor == nullOptions.BackOffDelayFactor {
		backOffDelay = defaultOptions.BackOffDelayFactor
	} else {
		backOffDelay = options.BackOffDelayFactor
	}

	effectiveOptions := FailAwareHTTPOptions{
		Timeout:            timeout,
		MaxRetries:         maxRetries,
		BackOffDelayFactor: backOffDelay,
		KeepLog:            options.KeepLog,
	}

	client := http.Client{
		Timeout: effectiveOptions.Timeout,
	}
	return &FailAwareHTTPClient{
		httpClient: &client,
		options:    effectiveOptions,
	}
}

//ErrEntry is used for logging retries and the result of retries.
type ErrEntry struct {
	err               error
	response          *http.Response
	timestampStarted  time.Time
	timestampFinished time.Time
}

func errEntryNow(err error, rsp *http.Response, started time.Time) ErrEntry {
	return ErrEntry{
		err:               err,
		response:          rsp,
		timestampStarted:  started,
		timestampFinished: time.Now(),
	}
}

//FailAwareHTTPError structured error returned by the FailAwareHTTP methods.
type FailAwareHTTPError struct {
	Retries   int
	Errors    []ErrEntry
	LastError error
}

func (e FailAwareHTTPError) Error() string {
	return fmt.Sprintf("err log: %#v", e.Errors)
}

//Post does a fail-aware Post request and retries in the case of retrieable errors
func (c *FailAwareHTTPClient) Post(url, contentType string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

//Do sends an arbitrary request and retries in the case of an retrieable error
func (c *FailAwareHTTPClient) Do(originalReq *http.Request) (*http.Response, error) {
	originalBody, err := readBody(originalReq.Body)
	defer func() {
		if originalReq.Body != nil {
			originalReq.Body.Close()
		}
	}()
	if err != nil {
		return nil, err
	}

	var lastResponse *http.Response
	var lastError error
	retried := 0
	var errLog []ErrEntry
	for ; retried < c.options.MaxRetries; retried++ {

		if originalBody != nil {
			reqBody := bytes.NewBuffer(originalBody)
			//just replace the body of the original request
			originalReq.Body = ioutil.NopCloser(reqBody)
		}

		started := time.Now()
		lastResponse, lastError = c.httpClient.Do(originalReq)
		if c.options.KeepLog {
			errLog = append(errLog, errEntryNow(lastError, lastResponse, started))
		}

		if lastError == nil && lastResponse.StatusCode < 500 && lastResponse.StatusCode != 429 {
			if lastError == nil {
				return lastResponse, nil
			}
			return lastResponse, FailAwareHTTPError{Retries: retried, Errors: errLog, LastError: lastError}
		}

		jitter := expJitterBackOff(retried, c.options.BackOffDelayFactor)

		<-time.After(jitter)
		log.Debugf("Retry #%d of request, waited %#v before retry", (retried + 1), jitter)
	}

	if lastError == nil {
		return lastResponse, nil
	}
	return lastResponse, FailAwareHTTPError{Retries: retried, Errors: errLog, LastError: lastError}
}

func readBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	strBody, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return strBody, nil
}

func expJitterBackOff(retries int, backOffDelayFactor time.Duration) time.Duration {
	exp := int(1 << uint(retries))
	ms := exp * int(backOffDelayFactor/time.Millisecond)
	maxJitter := ms / 3
	// ms ± rand
	ms += random.Intn(2*maxJitter) - maxJitter
	if ms <= 0 {
		ms = 1
	}
	return time.Duration(ms) * time.Millisecond
}
