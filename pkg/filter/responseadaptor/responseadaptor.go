package responseadaptor

import (
	"bytes"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

const (
	// Kind is the kind of ResponseAdaptor.
	Kind = "ResponseAdaptor"
)

var (
	results = []string{}
)

func init() {
	httppipeline.Register(&ResponseAdaptor{})
}

type (
	// ResponseAdaptor is filter ResponseAdaptor.
	ResponseAdaptor struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		Header *httpheader.AdaptSpec `yaml:"header" jsonschema:"required"`

		Body string `yaml:"body" jsonschema:"omitempty"`
	}
)

// Kind returns the kind of ResponseAdaptor.
func (ra *ResponseAdaptor) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of ResponseAdaptor.
func (ra *ResponseAdaptor) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of ResponseAdaptor.
func (ra *ResponseAdaptor) Description() string {
	return "ResponseAdaptor adapts response."
}

// Results returns the results of ResponseAdaptor.
func (ra *ResponseAdaptor) Results() []string {
	return results
}

// Init initializes ResponseAdaptor.
func (ra *ResponseAdaptor) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	ra.pipeSpec, ra.spec, ra.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	ra.reload()
}

// Inherit inherits previous generation of ResponseAdaptor.
func (ra *ResponseAdaptor) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	previousGeneration.Close()
	ra.Init(pipeSpec, super)
}

func (ra *ResponseAdaptor) reload() {
	// Nothing to do.
}

// Handle adapts response.
func (ra *ResponseAdaptor) Handle(ctx context.HTTPContext) string {
	result := ra.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (ra *ResponseAdaptor) handle(ctx context.HTTPContext) string {
	hte := ctx.Template()
	ctx.Response().Header().Adapt(ra.spec.Header, hte)

	if len(ra.spec.Body) != 0 {
		if hte.HasTemplates(ra.spec.Body) {
			if body, err := hte.Render(ra.spec.Body); err != nil {
				logger.Errorf("BUG responseadaptor render body faile , template %s , err %v",
					ra.spec.Body, err)
			} else {
				ctx.Response().SetBody(bytes.NewReader([]byte(body)))
			}
		} else {
			ctx.Response().SetBody(bytes.NewReader([]byte(ra.spec.Body)))
		}
	}
	return ""
}

// Status returns status.
func (ra *ResponseAdaptor) Status() interface{} { return nil }

// Close closes ResponseAdaptor.
func (ra *ResponseAdaptor) Close() {}