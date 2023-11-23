// ddanalyze prints the gRPC request parameters across spans in a Datadog trace.
//
// Usage:
//
//	ddanalyze [-tr trace_id] [-req grpc_request]
//
// grpc_request is the case-insensitive substring of a RPC name.
// The Datadog application key and API key should be present in DD_APP_KEY and
// DD_API_KEY environment variables.
//
// The traces downloaded from Datadog are cached as /tmp/ddanalyze-{trace_id}.
//
// The parameters are printed as JSON one per line to allow easy interaction with jq(1),
// sort(1), uniq(1) and other utilities.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ddanalyze [-tr trace_id] [-req grpc_request]\n")
	os.Exit(2)
}

var (
	traceId = flag.String("tr", "", "trace ID")
	grpcReq = flag.String("req", "", "gRPC request")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	if *traceId == "" || *grpcReq == "" {
		usage()
	}

	f := "/tmp/ddanalyze-" + *traceId
	var data []byte
	if _, err := os.Stat(f); err != nil {
		data, err = getTrace(*traceId)
		if err != nil {
			log.Fatal(err)
		}
		if err = os.WriteFile(f, data, 0444); err != nil {
			log.Fatal(err)
		}
	} else {
		data, err = os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
	}

	trace, err := prepareTrace(data)
	if err != nil {
		log.Fatal(err)
	}
	trace.printGrpcReqParams(*grpcReq)
}

type Trace struct {
	RootId string          `json:"root_id"`
	Spans  map[string]Span `json:"spans"`
}

type Span struct {
	SpanId      string   `json:"span_id"`
	ParentId    string   `json:"parent_id"`
	Start       float64  `json:"start"`
	Duration    float64  `json:"duration"`
	Service     string   `json:"service"`
	Name        string   `json:"name"`
	Resource    string   `json:"resource"`
	Meta        Meta     `json:"meta"`
	ChildrenIds []string `json:"children_ids"`
}

type Meta struct {
	GrpcRequest *string `json:"grpc.request"`
}

func (trace *Trace) getService(spanId string) string {
	return trace.Spans[spanId].Service
}

func (trace *Trace) getParent(spanId string) string {
	currId := spanId
	currService := trace.getService(spanId)
	for {
		parentId := trace.Spans[currId].ParentId
		parentService := trace.getService(parentId)
		if parentService != currService {
			return parentService
		}
		currId = parentId
	}
}

func (trace *Trace) printGrpcReqParams(grpcReq string) {
	if trace == nil {
		return
	}
	for _, span := range trace.Spans {
		if strings.Contains(
			strings.ToLower(span.Resource),
			strings.ToLower(grpcReq),
		) {
			if params := span.Meta.GrpcRequest; params != nil {
				parent := trace.getParent(span.SpanId)
				sec, _ := math.Modf(span.Start)
				t := time.Unix(int64(sec), 0)
				resp := map[string]any{
					"params":   *params,
					"parent":   parent,
					"start":    t.String(),
					"duration": span.Duration,
				}
				enc := json.NewEncoder(os.Stdout)
				if err := enc.Encode(resp); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func getTrace(traceId string) ([]byte, error) {
	apiKey := os.Getenv("DD_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DD_API_KEY environment variable not set")
	}
	appKey := os.Getenv("DD_APP_KEY")
	if appKey == "" {
		return nil, fmt.Errorf("DD_APP_KEY environment variable not set")
	}
	url := "https://app.datadoghq.com/api/v1/trace/" + traceId
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("DD-API-KEY", apiKey)
	req.Header.Add("DD-APPLICATION-KEY", appKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func prepareTrace(data []byte) (*Trace, error) {
	resp := struct {
		Trace *Trace `json:"trace"`
	}{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Trace == nil {
		return nil, fmt.Errorf("no trace found")
	}
	return resp.Trace, nil
}
