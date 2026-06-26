package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// cacheMaxAgeSeconds is how long clients may reuse a response without
// revalidating. The data refreshes server-side every 5-15 minutes; a short
// max-age keeps responses fresh while still absorbing bursts of polling.
const cacheMaxAgeSeconds = 60

// lastUpdatedGetter is implemented by every list/get response message (the
// generated protos expose GetLastUpdated()).
type lastUpdatedGetter interface {
	GetLastUpdated() *timestamppb.Timestamp
}

// cacheHeadersInterceptor adds Cache-Control (and Last-Modified when the
// response carries a lastUpdated) to read endpoints.
//
// It works through grpc-gateway's outgoing header matcher: response metadata
// keyed "grpc-metadata-<name>" is emitted as a clean HTTP header "<name>"
// (the same mechanism Prefab uses for x-http-code). Prefab does not expose a
// forward-response hook, so this interceptor is how we reach the HTTP layer.
func cacheHeadersInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err != nil || !isCacheableMethod(info.FullMethod) {
		return resp, err
	}

	pairs := []string{
		"grpc-metadata-cache-control", fmt.Sprintf("public, max-age=%d", cacheMaxAgeSeconds),
	}
	if lu, ok := resp.(lastUpdatedGetter); ok {
		if ts := lu.GetLastUpdated(); ts != nil {
			pairs = append(pairs, "grpc-metadata-last-modified", ts.AsTime().UTC().Format(http.TimeFormat))
		}
	}
	if mdErr := grpc.SetHeader(ctx, metadata.Pairs(pairs...)); mdErr != nil {
		// Non-fatal: caching headers are an optimization, not correctness.
		return resp, err
	}
	return resp, err
}

// isCacheableMethod reports whether a gRPC method is a safe, GET-mapped read
// whose response is backed by the TTL cache.
func isCacheableMethod(fullMethod string) bool {
	// e.g. "/api.v1.RoadsService/ListRoads"
	idx := strings.LastIndex(fullMethod, "/")
	if idx < 0 {
		return false
	}
	switch fullMethod[idx+1:] {
	case "ListRoads", "GetRoad", "ListIncidents",
		"ListWeather", "GetLocationWeather", "ListWeatherAlerts":
		return true
	default:
		return false
	}
}
