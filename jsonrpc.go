package gsmtcp

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/oklog/ulid"
	"github.com/rs/zerolog/log"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strconv"
	"time"
)

// ClientRequest represents a JSON-RPC request sent by a client.
type ClientRequest struct {
	JsonRpc string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  [1]interface{} `json:"params"`
	Id      string         `json:"id"`
}

// ClientResponse represents a JSON-RPC response returned to a client.
type ClientResponse struct {
	JsonRpc string           `json:"jsonrpc"`
	Result  *json.RawMessage `json:"result"`
	Error   interface{}      `json:"error"`
	Id      string           `json:"id"`
}

func generateULID() string {
	t := time.Now()
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

// EncodeClientRequest encodes parameters for a JSON-RPC client request.
func EncodeClientRequest(method string, args interface{}) ([]byte, error) {
	c := ClientRequest{
		JsonRpc: "2.0",
		Method:  method,
		Params:  [1]interface{}{args},
		Id:      generateULID(),
	}
	return json.Marshal(c)
}

// Error represents an arbitrary error value returned by a JSON-RPC request.
type Error struct {
	Data interface{}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v", e.Data)
}

// DoRequest executes a JSON-RPC request with the provided param. A pointer to the result reply
// must be given for the response to be unmarshalled.
func DoRequest(url string, conn *tls.Conn, method string, param interface{}, reply interface{}) error {
	c := &http.Client{
		Transport: Transport{Conn: conn},
	}
	request, err := EncodeClientRequest(method, param)
	if err != nil {
		log.Error()
		return err
	}
	r, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(request))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Length", strconv.Itoa(len(request)))
	r.Header.Add("Accept-Encoding", "identity")
	resp, err := c.Do(r)
	if err != nil {
		log.Error().Err(err)
		return err
	}
	if resp != nil {
		defer func() {
			if resp.Body != nil {
				err := resp.Body.Close()
				if err != nil {
					log.Error().Err(err).Msgf("could not close body: %s", err.Error())
				}
			}
		}()
		if resp.Body != nil {
			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Error().Err(err).Msgf("could not read resp.Body: %s", err.Error())
				return err
			}
			var c ClientResponse
			if err := json.NewDecoder(bytes.NewReader(content)).Decode(&c); err != nil {
				return err
			}
			if c.Error != nil {
				return &Error{Data: c.Error}
			}
			if c.Result == nil {
				return fmt.Errorf("unexpected null result")
			}
			return json.Unmarshal(*c.Result, reply)
		}
	}
	return nil
}

// Transport is a custom implementation of the Transport interface for utilising an existing TLS connection.
type Transport struct {
	Conn net.Conn
}

func (r Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBytes, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Error().Err(err)
		return nil, err
	}
	_, err = r.Conn.Write(reqBytes)
	if err != nil {
		log.Error().Err(err)
		return nil, err
	}
	// read the response
	scanner := bufio.NewScanner(r.Conn)
	// custom scanner that returns tokens between EOFs
	contentLengthRegexp := regexp.MustCompile("Content-Length: (78)")
	hasContentLength := func(b string) bool {
		return contentLengthRegexp.MatchString(b)
	}
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF {
			return 0, data, io.EOF
		}
		if isEndOfChunks(data) {
			return 0, data, bufio.ErrFinalToken
		}
		if hasContentLength(string(data)) {
			return 0, data, bufio.ErrFinalToken
		}
		return len(data), data, nil
	})
	// if data is available
	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}
	reader := bufio.NewReader(bytes.NewReader(data))
	var resp *http.Response
	resp, err = http.ReadResponse(reader, req)
	if err != nil {
		log.Error().Err(err).Msgf("could not read response body: %s", err.Error())
		return nil, err
	}
	return resp, nil
}

func isEndOfChunks(chunk []byte) bool {
	eoc := []byte{13, 10, 48, 13, 10, 13, 10}
	if len(chunk) == len(eoc) {
		for i, b := range chunk {
			if b != eoc[i] {
				return false
			}
		}
		return true
	}
	return false
}
