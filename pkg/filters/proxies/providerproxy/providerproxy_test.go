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
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
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
	rawSpec := make(map[string]interface{})
	err := codectool.Unmarshal([]byte(yamlConfig), &rawSpec)
	assert.NoError(err)

	spec, err := filters.NewSpec(nil, "", rawSpec)
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

	_ = req.FetchPayload(1024 * 1024)
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
  - https://ethereum-mainnet.s.chainbase.online
`
	proxy := newTestProviderProxy(yamlConfig, assert)

	postData := "{\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1,\"jsonrpc\":\"2.0\"}"

	stdr, _ := http.NewRequest(http.MethodPost, "https://www.megaease.com", bytes.NewReader([]byte(postData)))
	stdr.Header.Set("Content-Type", "application/json")
	ctx := getCtx(stdr)
	response := proxy.Handle(ctx)
	assert.Equal("", response)
	assert.NotNil(string(ctx.GetOutputResponse().RawPayload()))
	proxy.Close()
}
