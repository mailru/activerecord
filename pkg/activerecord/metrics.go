package activerecord

import "context"

type DefaultNoopMetric struct{}

func NewDefaultNoopMetric() *DefaultNoopMetric {
	return &DefaultNoopMetric{}
}

func (*DefaultNoopMetric) Timer(storage, entity string) MetricTimerInterface {
	return &DefaultNoopMetricTimer{}
}

func (*DefaultNoopMetric) StatCount(storage, entity string) MetricStatCountInterface {
	return &DefaultNoopMetricCount{}
}

func (*DefaultNoopMetric) ErrorCount(storage, entity string) MetricStatCountInterface {
	return &DefaultNoopMetricCount{}
}

type DefaultNoopMetricTimer struct{}

func (*DefaultNoopMetricTimer) Timing(ctx context.Context, name string) {}
func (*DefaultNoopMetricTimer) Finish(ctx context.Context, name string) {}

type DefaultNoopMetricCount struct{}

func (*DefaultNoopMetricCount) Inc(ctx context.Context, name string, val float64) {}
