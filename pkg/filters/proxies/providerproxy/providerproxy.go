package providerproxy

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

const (
	// Kind is the kind of ProviderProxy.
	Kind = "ProviderProxy"
)

type (
	ProviderProxy struct {
		spec   *Spec
		client *http.Client
	}

	Spec struct {
		filters.BaseSpec `json:",inline"`

		Urls []string `yaml:"urls"`
	}
)

func (m *ProviderProxy) SelectNode() (*url.URL, error) {
	urls := m.spec.Urls

	randomIndex := rand.Intn(len(urls))

	rpcUrl := urls[randomIndex]
	return url.Parse(rpcUrl)
}

func (m *ProviderProxy) Handle(ctx *context.Context) (result string) {
	reqUrl, err := m.SelectNode()
	fmt.Println("select rpc provider:", reqUrl.String())
	if err != nil {
		logger.Errorf(err.Error())
		return err.Error()
	}

	req := ctx.GetInputRequest().(*httpprot.Request)
	forwardReq, err := http.NewRequestWithContext(req.Context(), req.Method(), reqUrl.String(), req.GetPayload())
	response, err := m.client.Do(forwardReq)
	if err != nil {
		logger.Errorf(err.Error())
		return err.Error()
	}

	outputResponse, err := httpprot.NewResponse(response)
	outputResponse.Body = response.Body
	if err != nil {
		return err.Error()
	}

	if err = outputResponse.FetchPayload(1024 * 1024); err != nil {
		logger.Errorf("%s: failed to fetch response payload: %v, please consider to set serverMaxBodySize of SimpleHTTPProxy to -1.", m.Name(), err)
		return err.Error()
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
			Urls: make([]string, 0),
		}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &ProviderProxy{
			spec:   spec.(*Spec),
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
}

// Status returns status.
func (m *ProviderProxy) Status() interface{} { return nil }

// Close closes ProviderProxy.
func (m *ProviderProxy) Close() {}
