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

package selector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

type ProviderWeight struct {
	Url         string
	BlockNumber uint64
	Client      *RPCClient
}

type BlockLagProviderSelector struct {
	done      chan struct{}
	providers []ProviderWeight
	lag       uint64
	metrics   *metrics
}

func NewBlockLagProviderSelector(spec ProviderSelectorSpec, super *supervisor.Supervisor) ProviderSelector {

	providers := make([]ProviderWeight, 0)

	intervalDuration := spec.GetInterval()
	for _, value := range spec.Urls {
		client := &RPCClient{
			Endpoint: value,
			client: http.Client{
				Timeout: intervalDuration,
			},
		}
		providers = append(providers, ProviderWeight{
			Url:         value,
			BlockNumber: 0,
			Client:      client,
		})
	}

	ps := BlockLagProviderSelector{
		done:      make(chan struct{}),
		providers: providers,
		lag:       spec.Lag,
		metrics:   newMetrics(super),
	}
	ticker := time.NewTicker(intervalDuration)
	ps.checkServers()
	go func() {
		for {
			select {
			case <-ps.done:
				ticker.Stop()
				return
			case <-ticker.C:
				ps.checkServers()
			}
		}
	}()
	return ps
}

type ProviderBlock struct {
	index int
	block uint64
}

func (ps BlockLagProviderSelector) checkServers() {
	eg := new(errgroup.Group)
	blockNumberChannel := make(chan ProviderBlock, len(ps.providers))
	startTime := time.Now().Local()
	for i, _ := range ps.providers {
		eg.Go(func() error {
			client := ps.providers[i].Client
			req, _ := client.NewRequest("eth_getBlockByNumber", "latest", false)

			response, err := client.Send(req)
			if err != nil {
				blockNumberChannel <- ProviderBlock{
					index: i,
					block: 0,
				}
				return nil
			}

			head := types.Header{}
			err = json.Unmarshal(response, &head)
			if err != nil {
				blockNumberChannel <- ProviderBlock{
					index: i,
					block: 0,
				}
				return nil
			}

			blockNumberChannel <- ProviderBlock{
				index: i,
				block: head.Number.Uint64(),
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return
	}

	for i := 0; i < len(ps.providers); i++ {
		blockIndex := <-blockNumberChannel
		ps.providers[blockIndex.index].BlockNumber = blockIndex.block
		labels := prometheus.Labels{
			"provider": ps.providers[blockIndex.index].Url,
		}
		ps.metrics.ProviderBlockHeight.With(labels).Set(float64(blockIndex.block))
	}
	logger.Debugf("update block number time: %s", time.Since(startTime))
}

func (ps BlockLagProviderSelector) Close() {
	close(ps.done)
}

func (ps BlockLagProviderSelector) ChooseServer() (string, error) {

	if len(ps.providers) == 0 {
		return "", fmt.Errorf("no provider available")
	}

	if len(ps.providers) == 1 {
		return ps.providers[0].Url, nil
	}

	var bestProvider ProviderWeight
	for _, provider := range ps.providers {
		if provider.BlockNumber == 0 {
			continue
		}
		if provider.BlockNumber > bestProvider.BlockNumber &&
			(provider.BlockNumber-bestProvider.BlockNumber) >= ps.lag {
			bestProvider = provider
		}
	}

	if bestProvider.Url != "" {
		return bestProvider.Url, nil
	}

	return ps.providers[0].Url, nil
}

type metrics struct {
	ProviderBlockHeight *prometheus.GaugeVec
}

func newMetrics(super *supervisor.Supervisor) *metrics {
	commonLabels := prometheus.Labels{
		"pipelineName": super.Options().Name,
		"kind":         "BlockLagProviderSelector",
		"clusterName":  super.Options().ClusterName,
		"clusterRole":  super.Options().ClusterRole,
		"instanceName": super.Options().Name,
	}
	prometheusLabels := []string{
		"clusterName", "clusterRole", "instanceName", "pipelineName", "kind",
		"provider",
	}

	return &metrics{
		ProviderBlockHeight: prometheushelper.NewGauge(
			"provider_block_height",
			"the block height of provider", prometheusLabels).MustCurryWith(commonLabels),
	}
}
