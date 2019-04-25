package http

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const nonExistingURL = "http://localhost/doesNotExist" //an url that will always return an error

//Do

func TestRetriesDoOnRetrieableError(t *testing.T) {

	client := NewClient(optionsWithMinTimeouts())

	req, err := http.NewRequest("GET", nonExistingURL, nil)
	assert.Nil(t, err)

	_, err = client.Do(req)
	assert.NotNil(t, err)

	failErr := err.(FailAwareHTTPError)
	assert.Equal(t, 3, failErr.Retries)
}

func TestNoDoRetryOnNonRetrieableError(t *testing.T) {
	port, err := serverWith(400)
	if err != nil {
		t.Fatal("unable to start server", err)
	}
	url := fmt.Sprintf("http://localhost:%d", port)

	client := NewDefaultClient()
	req, err := http.NewRequest("GET", url, nil)
	assert.Nil(t, err)

	rsp, err := client.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 400, rsp.StatusCode)
}

func TestNoDoRetryOnOk(t *testing.T) {
	port, err := serverWith(200)
	if err != nil {
		t.Fatal("unable to start server", err)
	}
	url := fmt.Sprintf("http://localhost:%d", port)

	client := NewDefaultClient()
	req, err := http.NewRequest("GET", url, nil)
	assert.Nil(t, err)

	rsp, err := client.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode)
}

// Post

func TestRetriesPostOnRetrieableErrorWithTimeCheck(t *testing.T) {
	client := NewClient(optionsWithMinTimeouts())
	timeStarted := time.Now()
	_, err := client.Post(nonExistingURL, "application/json", strings.NewReader("dummyBody"))
	assert.NotNil(t, err)

	failErr := err.(FailAwareHTTPError)
	assert.Equal(t, 3, failErr.Retries)

	currentTime := timeStarted
	err0 := failErr.Errors[0]
	assert.NotNil(t, err0.err)
	assertTimeWithDiff(t, currentTime, err0.timestampStarted, 2*time.Millisecond)

	currentTime = currentTime.Add((5 + 2) * time.Millisecond)
	err1 := failErr.Errors[1]
	assert.NotNil(t, err1.err)
	assertTimeWithDiff(t, currentTime, err1.timestampStarted, 4*time.Millisecond)

	currentTime = currentTime.Add((10 + 4) * time.Millisecond)
	err2 := failErr.Errors[2]
	assert.NotNil(t, err2.err)
	assertTimeWithDiff(t, currentTime, err2.timestampStarted, 10*time.Millisecond)
}

func TestNoPostRetryOnNonRetrieableError(t *testing.T) {
	port, err := serverWith(400)
	if err != nil {
		t.Fatal("unable to start server", err)
	}
	url := fmt.Sprintf("http://localhost:%d", port)

	client := NewDefaultClient()
	rsp, err := client.Post(url, "application/json", strings.NewReader("dummyBody"))
	assert.Nil(t, err)
	assert.Equal(t, 400, rsp.StatusCode)
}

func TestNoPostRetryOnOk(t *testing.T) {
	port, err := serverWith(200)
	if err != nil {
		t.Fatal("unable to start server", err)
	}
	url := fmt.Sprintf("http://localhost:%d", port)

	client := NewDefaultClient()
	rsp, err := client.Post(url, "application/json", strings.NewReader("dummyBody"))
	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode)
}

//Helper

func optionsWithMinTimeouts() FailAwareHTTPOptions {
	return FailAwareHTTPOptions{
		MaxRetries:         3,
		Timeout:            10 * time.Millisecond,
		KeepLog:            true,
		BackOffDelayFactor: 5 * time.Millisecond,
	}
}

func assertTimeWithDiff(t *testing.T, expected, actual time.Time, diffMax time.Duration) {
	diffActual := absi(expected.UnixNano() - actual.UnixNano())
	assert.True(t, int64(diffActual) < int64(diffMax), fmt.Sprintf("max time diff exceeded, was %s, max allowed %s", time.Duration(diffActual), diffMax))
}

func serverWith(statusCode int) (int, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(fmt.Sprintf("%d status code", statusCode)))
	})
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, fmt.Errorf("unable to secure listener %v", err)
	}
	go func() {
		if errSrv := http.Serve(l, mux); errSrv != nil {
			log.Fatalf("slow-server error %v", errSrv)
		}
	}()

	var port int
	_, sport, err := net.SplitHostPort(l.Addr().String())
	if err == nil {
		port, err = strconv.Atoi(sport)
	}

	if err != nil {
		return -1, fmt.Errorf("unable to determine port %v", err)
	}

	return port, nil
}

func absi(i int64) int64 {
	if i < 0 {
		return i * -1
	}
	return i
}