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
	assert.Equal([]string{"eth_blockNumber"}, method)

	method = proxy.ParsePayloadMethod([]byte("{\"method\":\"eth_getBlockByNumber\",\"params\":[\"0xc5043f\",false],\"id\":1,\"jsonrpc\":\"2.0\"}"))
	assert.Equal([]string{"eth_getBlockByNumber"}, method)

	method = proxy.ParsePayloadMethod([]byte("test unknown payload"))
	assert.Equal([]string{"UNKNOWN"}, method)

	method = proxy.ParsePayloadMethod([]byte{})
	assert.Equal([]string{"UNKNOWN"}, method)

	method = proxy.ParsePayloadMethod([]byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"suix_getAllBalances\",\"params\":[\"0x94f1a597b4e8f709a396f7f6b1482bdcd65a673d111e49286c527fab7c2d0961\"]}"))
	assert.Equal([]string{"suix_getAllBalances"}, method)

	method = proxy.ParsePayloadMethod([]byte("[{\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x7363bf80269875c6ddd3de0089baf0a9af28586dd0e536753d1cbb5eb9d6535b\"], \"id\": 0}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x29696eba0fa0eb5eb4c1495174c6fb37a9e64a3707dc8b64c9d19a650c8e1b5f\"], \"id\": 1}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x8f9631d0fd6a056e422ed0eea938a84b1eca3344729789f4465861c6406c2114\"], \"id\": 2}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x4787c3d707ff6d204590c77f672917dca49c36ec0381d6dea10d84981d008ec5\"], \"id\": 3}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x11132ec86262aa67b27f64a5f5a6550c7b86f85dabe3d824dbab0f20283f9aa6\"], \"id\": 4}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x8efac98473a7cdb5c40ca503089794b06723c09791833c2df5b4732ea5a10451\"], \"id\": 5}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x7df0d4c1b2d1a570a1367a8bd09301f8e6af5ffa03a2b97a693ba440bb87b002\"], \"id\": 6}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x7d473c856b0b8d207c43e46a3327217e809142ad5497109d01ec72d6a1bde45c\"], \"id\": 7}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x512013ff4a44b6604b81f63e330d57a63b03f23dae011753be8c168ce8f6fcc7\"], \"id\": 8}, {\"jsonrpc\": \"2.0\", \"method\": \"eth_getTransactionReceipt\", \"params\": [\"0x438abc4bfc9a46296f191ad3ecc742bbd7da9286a2a92fe8f27f2d0168a19661\"], \"id\": 9}]"))
	assert.Equal([]string{"eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_getTransactionReceipt"}, method)

	proxy.Close()
}
