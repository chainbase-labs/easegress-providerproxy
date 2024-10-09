package providerproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
)

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

type RPCClient struct {
	client    http.Client
	Endpoint  string
	idCounter atomic.Uint32
}

func (hc *RPCClient) nextID() json.RawMessage {
	id := hc.idCounter.Add(1)
	return strconv.AppendUint(nil, uint64(id), 10)
}

func (hc *RPCClient) NewRequest(method string, paramsIn ...interface{}) (*http.Request, error) {
	msg := &jsonrpcMessage{Version: "2.0", ID: hc.nextID(), Method: method}
	if paramsIn != nil { // prevent sending "params":null
		var err error
		if msg.Params, err = json.Marshal(paramsIn); err != nil {
			return nil, err
		}
	}

	jsonBody, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	bodyReader := bytes.NewReader(jsonBody)

	req, err := http.NewRequest(http.MethodPost, hc.Endpoint, bodyReader)
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (hc *RPCClient) Send(req *http.Request) (json.RawMessage, error) {
	res, err := hc.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	resp := jsonrpcMessage{}
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}
