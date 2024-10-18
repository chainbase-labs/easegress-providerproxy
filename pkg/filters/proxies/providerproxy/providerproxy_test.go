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
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func newTestProviderProxy(yamlConfig string, assert *assert.Assertions) *ProviderProxy {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Recovered from panic: %v\n", err)
		}
	}()

	rawSpec := make(map[string]interface{})
	err := codectool.Unmarshal([]byte(yamlConfig), &rawSpec)
	assert.NoError(err)

	opt := option.New()
	opt.Name = "test"
	opt.ClusterName = "test"
	opt.ClusterRole = "secondary"

	super := supervisor.NewMock(opt, nil, nil,
		nil, false, nil, nil)

	spec, err := filters.NewSpec(super, "", rawSpec)
	assert.NoError(err)

	proxy := kind.CreateInstance(spec).(*ProviderProxy)

	proxy.Init()

	assert.Equal(kind, proxy.Kind())
	assert.Equal(spec, proxy.Spec())
	return proxy
}

func getCtx(stdr *http.Request) *context.Context {
	req, _ := httpprot.NewRequest(stdr)
	for key := range stdr.Header {
		req.HTTPHeader().Set(key, stdr.Header.Get(key))
	}

	err := req.FetchPayload(1024 * 1024)
	if err != nil {
		logger.Errorf(err.Error())
	}
	ctx := context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, req)
	return ctx
}

func TestProviderProxy(t *testing.T) {
	assert := assert.New(t)

	const yamlConfig = `
name: providerProxy
kind: ProviderProxy
urls:
  - https://eth.llamarpc.com
`
	proxy := newTestProviderProxy(yamlConfig, assert)

	postData := "{\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1,\"jsonrpc\":\"2.0\"}"

	stdr, _ := http.NewRequest(http.MethodPost, "https://www.megaease.com", strings.NewReader(postData))
	stdr.Header.Set("Content-Type", "application/json")
	ctx := getCtx(stdr)
	response := proxy.Handle(ctx)
	assert.Equal("", response)
	assert.NotNil(ctx.GetResponse(context.DefaultNamespace).GetPayload())

	proxy.Close()
}

func TestProviderProxy_ParsePayloadMethod(t *testing.T) {
	assert := assert.New(t)

	const yamlConfig = `
name: providerProxy
kind: ProviderProxy
urls:
  - https://eth.llamarpc.com
`
	proxy := newTestProviderProxy(yamlConfig, assert)

	method := proxy.ParsePayloadMethod([]byte("{\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1,\"jsonrpc\":\"2.0\"}"))
	assert.Equal("eth_blockNumber", method)

	method = proxy.ParsePayloadMethod([]byte("{\"method\":\"eth_getBlockByNumber\",\"params\":[\"0xc5043f\",false],\"id\":1,\"jsonrpc\":\"2.0\"}"))
	assert.Equal("eth_getBlockByNumber", method)

	method = proxy.ParsePayloadMethod([]byte("test unknown payload"))
	assert.Equal("UNKNOWN", method)

	method = proxy.ParsePayloadMethod([]byte{})
	assert.Equal("UNKNOWN", method)

	method = proxy.ParsePayloadMethod([]byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"suix_getAllBalances\",\"params\":[\"0x94f1a597b4e8f709a396f7f6b1482bdcd65a673d111e49286c527fab7c2d0961\"]}"))
	assert.Equal("suix_getAllBalances", method)

	proxy.Close()
}
