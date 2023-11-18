package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ddanalyze [-tr trace_id] [-req grpc_request]\n")
	os.Exit(2)
}

var (
	traceId = flag.String("tr", "", "trace ID")
	grpcReq = flag.String("req", "", "grpc request")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) != 0 {
		fmt.Println(args)
		usage()
	}

	data, err := getBody(*traceId)
	if err != nil {
		log.Fatal(err)
	}
	trace, err := prepare(data)
	if err != nil {
		log.Fatal(err)
	}
	trace.printGrpcRequests(*grpcReq)
}

type Trace struct {
	RootId *string          `json:"root_id,omitempty"`
	Spans  map[string]*Span `json:"spans,omitempty"`
}

type Span struct {
	SpanId      *string  `json:"span_id,omitempty"`
	ParentId    *string  `json:"parent_id,omitempty"`
	Service     *string  `json:"service,omitempty"`
	Name        *string  `json:"name,omitempty"`
	Resource    *string  `json:"resource,omitempty"`
	Meta        *Meta    `json:"meta,omitempty"`
	ChildrenIds []string `json:"children_ids,omitempty"`
}

type Meta struct {
	GrpcRequest *string `json:"grpc.request,omitempty"`
}

func (trace *Trace) getService(spanId *string) string {
	return *trace.Spans[*spanId].Service
}

func (trace *Trace) getParent(spanId *string) string {
	currId := spanId
	currService := trace.getService(spanId)
	for {
		parentId := trace.Spans[*currId].ParentId
		parentService := trace.getService(parentId)
		if parentService != currService {
			return parentService
		}
		currId = parentId
	}
}

func (trace *Trace) printGrpcRequests(req string) {
	if trace == nil {
		return
	}
	for _, span := range trace.Spans {
		if span.Resource == nil {
			continue
		}
		if strings.Contains(*span.Resource, req) {
			if grpcRequest := span.Meta.GrpcRequest; grpcRequest != nil {
				parent := trace.getParent(span.SpanId)
				fmt.Printf("req=%v, parent=%v\n", *grpcRequest, parent)
			}
		}
	}
}

func getBody(traceId string) ([]byte, error) {
	url := "https://app.datadoghq.com/api/v1/trace/" + traceId
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("DD-API-KEY", os.Getenv("DD_API_KEY"))
	req.Header.Add("DD-APPLICATION-KEY", os.Getenv("DD_APP_KEY"))
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

func prepare(data []byte) (*Trace, error) {
	resp := struct {
		Trace *Trace `json:"trace,omitempty"`
	}{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Trace, nil
}
