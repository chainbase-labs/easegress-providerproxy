/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package providerproxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	proxy "github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/providerproxy/selector"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
	"github.com/megaease/easegress/v2/pkg/util/readers"
)

const (
	// Kind is the kind of ProviderProxy.
	Kind = "ProviderProxy"
)

type (
	ProviderProxy struct {
		super            *supervisor.Supervisor
		spec             *Spec
		client           *http.Client
		providerSelector selector.ProviderSelector
		metrics          *metrics
	}

	Spec struct {
		filters.BaseSpec `json:",inline"`

		Urls     []string `yaml:"urls"`
		Interval string   `yaml:"interval,omitempty" jsonschema:"format=duration"`
		Lag      uint64   `yaml:"lag,omitempty" jsonschema:"default=100"`
		Policy   string   `yaml:"policy,omitempty" jsonschema:"default=roundRobin"`

		MaxIdleConns        int `json:"maxIdleConns,omitempty"`
		MaxIdleConnsPerHost int `json:"maxIdleConnsPerHost,omitempty"`
		MaxRedirection      int `json:"maxRedirection,omitempty"`
	}
)

func (m *ProviderProxy) SelectNode() (*url.URL, error) {
	rpcUrl, err := m.providerSelector.ChooseServer()
	if err != nil {
		return nil, err
	}
	return url.Parse(rpcUrl)
}

func (m *ProviderProxy) ParsePayloadMethod(payload []byte) []string {
	defaultValue := []string{"UNKNOWN"}
	if len(payload) <= 0 {
		return defaultValue
	}

	jsonBody := map[string]interface{}{}
	err := json.Unmarshal(payload, &jsonBody)
	if err == nil {
		method, exists := jsonBody["method"].(string)
		if !exists {
			return defaultValue
		}
		return []string{method}
	}

	// parse batch call json array
	var jsonBodyArr []map[string]interface{}
	err = json.Unmarshal(payload, &jsonBodyArr)
	if err != nil {
		logger.Errorf("parse batch call err: %s, Body: %s", err, string(payload))
		return defaultValue
	}

	methods := make([]string, 0)

	for _, item := range jsonBodyArr {
		method, exists := item["method"].(string)
		if !exists {
			methods = append(methods, "UNKNOWN")
		}
		methods = append(methods, method)
	}
	return methods
}

func (m *ProviderProxy) HandleRequest(req *httpprot.Request, providerUrl *url.URL) (forwardReq *http.Request, method []string, err error) {
	if len(req.URL().Path) != 0 && req.URL().Path != "/" {
		providerUrl = providerUrl.JoinPath(req.URL().Path)
		pathMethod := strings.Replace(req.URL().Path, "//", "/", -1)
		method = []string{pathMethod}
	} else {
		bodyBytes := req.RawPayload()
		method = m.ParsePayloadMethod(bodyBytes)
	}

	forwardReq, err = http.NewRequestWithContext(req.Context(), req.Method(), providerUrl.String(), req.GetPayload())
	return
}

func (m *ProviderProxy) Handle(ctx *context.Context) (result string) {
	requestMetrics := RequestMetrics{}

	startTime := fasttime.Now()
	reqUrl, err := m.SelectNode()
	if err != nil {
		logger.Errorf(err.Error())
		return err.Error()
	}

	logger.Infof("select rpc provider: %s", reqUrl.String())
	requestMetrics.Provider = reqUrl.String()
	req := ctx.GetInputRequest().(*httpprot.Request)
	forwardReq, method, err := m.HandleRequest(req, reqUrl)

	if err != nil {
		logger.Errorf(err.Error())
		return err.Error()
	}

	for key := range req.HTTPHeader() {
		forwardReq.Header.Add(key, req.HTTPHeader().Get(key))
	}

	response, err := m.client.Do(forwardReq)

	if err != nil {
		logger.Errorf(err.Error())
		return err.Error()
	}

	requestMetrics.RpcMethod = method
	requestMetrics.Duration = fasttime.Since(startTime)
	requestMetrics.StatusCode = response.StatusCode
	defer m.collectMetrics(requestMetrics)

	body := readers.NewCallbackReader(response.Body)
	response.Body = body
	outputResponse, err := httpprot.NewResponse(response)

	if err != nil {
		logger.Errorf(err.Error())
		response.Body.Close()
		return err.Error()
	}

	if err = outputResponse.FetchPayload(-1); err != nil {
		logger.Errorf("%s: failed to fetch response payload: %v, please consider to set serverMaxBodySize of SimpleHTTPProxy to -1.", m.Name(), err)
		response.Body.Close()
		return err.Error()
	}

	if !outputResponse.IsStream() {
		response.Body.Close()
	}
	ctx.SetResponse(context.DefaultNamespace, outputResponse)
	return ""
}

var kind = &filters.Kind{
	Name:        Kind,
	Description: "ProviderProxy",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &Spec{
			Urls:     make([]string, 0),
			Interval: "1s",
			Policy:   "roundRobin",

			MaxIdleConns:        10240,
			MaxIdleConnsPerHost: 1024,
		}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		providerSpec := spec.(*Spec)
		return &ProviderProxy{
			spec:   providerSpec,
			super:  spec.Super(),
			client: http.DefaultClient,
		}
	},
}

func init() { filters.Register(kind) }

// Name returns the name of the HeaderCounter filter instance.
func (m *ProviderProxy) Name() string { return m.spec.Name() }

// Kind returns the kind of ProviderProxy.
func (m *ProviderProxy) Kind() *filters.Kind { return kind }

// Spec returns the spec used by the ProviderProxy.
func (m *ProviderProxy) Spec() filters.Spec { return m.spec }

// Init initializes ProviderProxy.
func (m *ProviderProxy) Init() {
	urls := m.spec.Urls
	if len(urls) == 0 {
		panic(errors.New("node address not provided"))
	}
	m.reload()
}

// Inherit inherits previous generation of ProviderProxy.
func (m *ProviderProxy) Inherit(previousGeneration filters.Filter) {
	m.Init()
}

func (m *ProviderProxy) reload() {
	clientSpec := &proxy.HTTPClientSpec{
		MaxIdleConns:        m.spec.MaxIdleConns,
		MaxIdleConnsPerHost: m.spec.MaxIdleConnsPerHost,
		MaxRedirection:      &m.spec.MaxRedirection,
	}

	client := proxy.HTTPClient(nil, clientSpec, 0)
	m.client = client

	providerSelectorSpec := selector.ProviderSelectorSpec{
		Name:     m.spec.BaseSpec.Name(),
		Urls:     m.spec.Urls,
		Interval: m.spec.Interval,
		Lag:      m.spec.Lag,
	}

	m.metrics = m.newMetrics()

	providerSelector := selector.CreateProviderSelectorByPolicy(m.spec.Policy, providerSelectorSpec)
	m.providerSelector = providerSelector
}

// Status returns status.
func (m *ProviderProxy) Status() interface{} { return nil }

// Close closes ProviderProxy.
func (m *ProviderProxy) Close() {
	if m.providerSelector != nil {
		ps := m.providerSelector
		m.providerSelector = nil
		ps.Close()
	}
}
