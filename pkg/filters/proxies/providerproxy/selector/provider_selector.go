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
	"time"
)

type ProviderSelectorSpec struct {
	Name     string   `json:"name"`
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

type ProviderSelector interface {
	ChooseServer() (string, error)
	Close()
}

func CreateProviderSelectorByPolicy(policy string, spec ProviderSelectorSpec) ProviderSelector {
	switch policy {
	case "blockLag":
		return NewBlockLagProviderSelector(spec)
	case "roundRobin":
		return NewRoundRobinProviderSelector(spec)
	default:
		return NewRoundRobinProviderSelector(spec)
	}
}
