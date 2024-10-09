package providerproxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/megaease/easegress/v2/pkg/logger"
	"golang.org/x/sync/errgroup"
)

type ProviderSelectorSpec struct {
	Urls     []string `json:"urls"`
	Interval string   `json:"interval,omitempty" jsonschema:"format=duration"`
	Lag      uint64   `json:"lag,omitempty" jsonschema:"default=100"`
}

// GetInterval returns the interval duration.
func (ps *ProviderSelectorSpec) GetInterval() time.Duration {
	interval, _ := time.ParseDuration(ps.Interval)
	if interval <= 0 {
		interval = time.Second
	}
	return interval
}

type ProviderWeight struct {
	Url         string
	BlockNumber uint64
	Client      *RPCClient
}

type ProviderSelector struct {
	done      chan struct{}
	providers []ProviderWeight
	lag       uint64
}

func NewProviderSelector(spec ProviderSelectorSpec) ProviderSelector {

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

	ps := ProviderSelector{
		done:      make(chan struct{}),
		providers: providers,
		lag:       spec.Lag,
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

func (ps ProviderSelector) checkServers() {
	log.Println("check block number")
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
	}
	logger.Debugf("update block number time: %s", time.Since(startTime))
}

func (ps ProviderSelector) Close() {
	close(ps.done)
}

func (ps ProviderSelector) ChooseServer() (string, error) {

	if len(ps.providers) == 0 {
		return "", fmt.Errorf("no provider available")
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
