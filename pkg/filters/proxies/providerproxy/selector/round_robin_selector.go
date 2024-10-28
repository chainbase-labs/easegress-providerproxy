package selector

import (
	"fmt"
	"math/rand"
)

type RoundRobinProviderSelector struct {
	providers []string
}

func (ps *RoundRobinProviderSelector) ChooseServer() (string, error) {
	if len(ps.providers) == 0 {
		return "", fmt.Errorf("no provider available")
	}

	urls := ps.providers
	randomIndex := rand.Intn(len(urls))
	rpcUrl := urls[randomIndex]
	return rpcUrl, nil
}

func (ps *RoundRobinProviderSelector) Close() {
	// do nothing
}

func NewRoundRobinProviderSelector(spec ProviderSelectorSpec) ProviderSelector {
	return &RoundRobinProviderSelector{
		providers: spec.Urls,
	}
}
