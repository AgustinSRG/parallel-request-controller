// Client test

package prc_client

import (
	"os"
	"sync"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func getEnvString(key string, defaultVal string) string {
	v := os.Getenv(key)

	if v == "" {
		v = defaultVal
	}

	return v
}

type testErrorHandler struct {
	t *testing.T
}

func (th *testErrorHandler) OnConnectionError(err error) {
	th.t.Log("Connection error: " + err.Error())
}

func (th *testErrorHandler) OnServerError(code string, message string) {
	th.t.Error("Server error: " + code + ": " + message)
}

func testStartRequest(t *testing.T, wg *sync.WaitGroup, startedRequestsArray []*StartedRequest, limitedArray []bool, index int, cli *Client, rType string, limit uint32) {
	defer wg.Done()

	r, limited, err := cli.StartRequest(rType, limit)

	if err != nil {
		t.Error(err)
		return
	}

	limitedArray[index] = limited
	startedRequestsArray[index] = r
}

func TestClient(t *testing.T) {
	godotenv.Load() // Load env vars

	th := &testErrorHandler{
		t: t,
	}

	cli := NewClient(&ClientConfig{
		Url:          getEnvString("SERVER_URL", "ws://localhost:8080"),
		AuthToken:    getEnvString("AUTH_TOKEN", "change_me"),
		ErrorHandler: th,
	})

	cli.Connect()

	startedRequestsArray := []*StartedRequest{nil, nil, nil, nil, nil}
	limitedArray := []bool{false, false, false, false, false}

	wg := &sync.WaitGroup{}
	wg.Add(5)

	go testStartRequest(t, wg, startedRequestsArray, limitedArray, 0, cli, "test-type-1", 2)
	go testStartRequest(t, wg, startedRequestsArray, limitedArray, 1, cli, "test-type-1", 2)
	go testStartRequest(t, wg, startedRequestsArray, limitedArray, 2, cli, "test-type-1", 2)

	go testStartRequest(t, wg, startedRequestsArray, limitedArray, 3, cli, "test-type-2", 2)
	go testStartRequest(t, wg, startedRequestsArray, limitedArray, 4, cli, "test-type-2", 2)

	wg.Wait()

	limitedCount := 0

	for i := 0; i < 3; i++ {
		if limitedArray[i] {
			limitedCount++
		}
	}

	assert.Equal(t, limitedCount, 1)

	limitedCount = 0

	for i := 3; i < 5; i++ {
		if limitedArray[i] {
			limitedCount++
		}
	}

	assert.Equal(t, limitedCount, 0)

	// Count requests

	count, err := cli.GetRequestCount("test-type-1")

	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, count, uint32(2))

	count, err = cli.GetRequestCount("test-type-2")

	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, count, uint32(2))

	// End requests

	for _, sr := range startedRequestsArray {
		if sr != nil {
			go sr.End()
		}
	}

	// Wait for counts to be 0

	done := false

	for !done {
		count, err := cli.GetRequestCount("test-type-1")

		if err != nil {
			t.Error(err)
			return
		}

		if count > 0 {
			continue
		}

		count, err = cli.GetRequestCount("test-type-2")

		if err != nil {
			t.Error(err)
			return
		}

		if count > 0 {
			continue
		}

		done = true
	}

	cli.Close()
}
