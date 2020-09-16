package tracer

import (
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/pkg/errors"
)

type ITracer interface {
	GetTracer() opentracing.Tracer
	GetParentSpan() opentracing.Span
	GetChildSpan() opentracing.Span
	IsParentSpan() bool
	IsChildSpan() bool

	Parent(req *http.Request)
	Child(req *http.Request) error
	ExtURL(span opentracing.Span, method string, url string)
	ExtStatus(span opentracing.Span, status int)
	Finish()
}

type Tracer struct {
	tracer     opentracing.Tracer
	parentSpan opentracing.Span
	childSpan  opentracing.Span
}

func NewTracer(tracer opentracing.Tracer) *Tracer {
	opentracing.SetGlobalTracer(tracer)
	return &Tracer{
		tracer: opentracing.GlobalTracer(),
	}
}

func (t *Tracer) Finish() {
	if t.parentSpan != nil {
		t.parentSpan.Finish()
		t.parentSpan = nil
	}

	if t.childSpan != nil {
		t.childSpan.Finish()
		t.childSpan = nil
	}
}

func (t *Tracer) IsParentSpan() bool {
	return t.parentSpan != nil
}

func (t *Tracer) IsChildSpan() bool {
	return t.childSpan != nil
}

func (t *Tracer) Parent(req *http.Request) {
	spanCtx, _ := t.tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	t.parentSpan = t.tracer.StartSpan(req.URL.Path, ext.RPCServerOption(spanCtx))
}

func (t *Tracer) Child(req *http.Request) error {
	if t.parentSpan == nil {
		return errors.New("not find opentracing.SpanContext")
	}

	t.childSpan = t.tracer.StartSpan(req.URL.Path, opentracing.ChildOf(t.parentSpan.Context()))
	return nil
}

func (t *Tracer) ExtStatus(span opentracing.Span, status int) {
	ext.HTTPStatusCode.Set(span, uint16(status))
}

func (t *Tracer) ExtURL(span opentracing.Span, method, url string) {
	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, url)
	ext.HTTPMethod.Set(span, method)
}

func (t *Tracer) GetTracer() opentracing.Tracer {
	return t.tracer
}

func (t *Tracer) GetParentSpan() opentracing.Span {
	return t.parentSpan
}

func (t *Tracer) GetChildSpan() opentracing.Span {
	return t.childSpan
}
