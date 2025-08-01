// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelgrpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/semconv/v1.34.0/rpcconv"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

func TestStatsHandlerHandleRPCServerErrors(t *testing.T) {
	for _, check := range serverChecks {
		name := check.grpcCode.String()
		t.Run(name, func(t *testing.T) {
			t.Setenv("OTEL_METRICS_EXEMPLAR_FILTER", "always_off")
			sr := tracetest.NewSpanRecorder()
			tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))

			mr := metric.NewManualReader()
			mp := metric.NewMeterProvider(metric.WithReader(mr))

			serverHandler := otelgrpc.NewServerHandler(
				otelgrpc.WithTracerProvider(tp),
				otelgrpc.WithMeterProvider(mp),
				otelgrpc.WithMetricAttributes(testMetricAttr),
			)

			serviceName := "TestGrpcService"
			methodName := serviceName + "/" + name
			fullMethodName := "/" + methodName
			// call the server handler
			ctx := serverHandler.TagRPC(context.Background(), &stats.RPCTagInfo{
				FullMethodName: fullMethodName,
			})

			grpcErr := status.Error(check.grpcCode, check.grpcCode.String())
			serverHandler.HandleRPC(ctx, &stats.End{
				Error: grpcErr,
			})

			// validate span
			span, ok := getSpanFromRecorder(sr, methodName)
			require.True(t, ok, "missing span %s", methodName)
			assertServerSpan(t, check.wantSpanCode, check.wantSpanStatusDescription, check.grpcCode, span)

			// validate metric
			assertStatsHandlerServerMetrics(t, mr, serviceName, name, check.grpcCode)
		})
	}
}

func assertStatsHandlerServerMetrics(t *testing.T, reader metric.Reader, serviceName, name string, code codes.Code) {
	want := metricdata.ScopeMetrics{
		Scope: wantInstrumentationScope,
		Metrics: []metricdata.Metrics{
			{
				Name:        rpcconv.ServerDuration{}.Name(),
				Description: rpcconv.ServerDuration{}.Description(),
				Unit:        rpcconv.ServerDuration{}.Unit(),
				Data: metricdata.Histogram[float64]{
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.HistogramDataPoint[float64]{
						{
							Attributes: attribute.NewSet(
								semconv.RPCMethod(name),
								semconv.RPCService(serviceName),
								semconv.RPCSystemGRPC,
								semconv.RPCGRPCStatusCodeKey.Int64(int64(code)),
								testMetricAttr,
							),
						},
					},
				},
			},
			{
				Name:        rpcconv.ServerRequestsPerRPC{}.Name(),
				Description: rpcconv.ServerRequestsPerRPC{}.Description(),
				Unit:        rpcconv.ServerRequestsPerRPC{}.Unit(),
				Data: metricdata.Histogram[int64]{
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.HistogramDataPoint[int64]{
						{
							Attributes: attribute.NewSet(
								semconv.RPCMethod(name),
								semconv.RPCService(serviceName),
								semconv.RPCSystemGRPC,
								semconv.RPCGRPCStatusCodeKey.Int64(int64(code)),
								testMetricAttr,
							),
						},
					},
				},
			},
			{
				Name:        rpcconv.ServerResponsesPerRPC{}.Name(),
				Description: rpcconv.ServerResponsesPerRPC{}.Description(),
				Unit:        rpcconv.ServerResponsesPerRPC{}.Unit(),
				Data: metricdata.Histogram[int64]{
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.HistogramDataPoint[int64]{
						{
							Attributes: attribute.NewSet(
								semconv.RPCMethod(name),
								semconv.RPCService(serviceName),
								semconv.RPCSystemGRPC,
								semconv.RPCGRPCStatusCodeKey.Int64(int64(code)),
								testMetricAttr,
							),
						},
					},
				},
			},
		},
	}
	rm := metricdata.ResourceMetrics{}
	err := reader.Collect(context.Background(), &rm)
	assert.NoError(t, err)
	require.Len(t, rm.ScopeMetrics, 1)
	metricdatatest.AssertEqual(t, want, rm.ScopeMetrics[0], metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
}
