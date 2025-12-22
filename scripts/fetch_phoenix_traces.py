#!/usr/bin/env python3
"""
Fetch traces from Arize Phoenix for a specific elastic-package session.

Usage:
    python fetch_phoenix_traces.py <session-id> [--output traces.json]

Environment variables:
    PHOENIX_BASE_URL: Phoenix server URL (default: http://localhost:6006)
"""

import argparse
import json
import os
import sys

import requests


def fetch_traces(session_id: str, base_url: str) -> dict:
    """Fetch all spans for a given session ID from Phoenix using GraphQL."""
    graphql_url = f"{base_url}/graphql"
    
    # GraphQL query to fetch session, traces, and spans
    query = """
    query GetSessionTraces($sessionId: String!) {
      getProjectSessionById(sessionId: $sessionId) {
        sessionId
        numTraces
        startTime
        endTime
        traces(first: 100) {
          edges {
            node {
              traceId
              startTime
              endTime
              latencyMs
              spans(first: 1000) {
                edges {
                  node {
                    name
                    spanKind
                    statusCode
                    statusMessage
                    startTime
                    endTime
                    latencyMs
                    parentId
                    tokenCountTotal
                    tokenCountPrompt
                    tokenCountCompletion
                    input {
                      value
                    }
                    output {
                      value
                    }
                    attributes
                  }
                }
              }
            }
          }
        }
      }
    }
    """
    
    response = requests.post(
        graphql_url,
        json={"query": query, "variables": {"sessionId": session_id}},
        headers={"Content-Type": "application/json"},
    )
    response.raise_for_status()
    
    data = response.json()
    
    if "errors" in data:
        raise Exception(f"GraphQL errors: {data['errors']}")
    
    session_data = data.get("data", {}).get("getProjectSessionById")
    if not session_data:
        return None
    
    # Flatten the structure for easier reading
    result = {
        "sessionId": session_data["sessionId"],
        "numTraces": session_data["numTraces"],
        "startTime": session_data["startTime"],
        "endTime": session_data["endTime"],
        "traces": [],
    }
    
    for trace_edge in session_data.get("traces", {}).get("edges", []):
        trace = trace_edge["node"]
        trace_data = {
            "traceId": trace["traceId"],
            "startTime": trace["startTime"],
            "endTime": trace["endTime"],
            "latencyMs": trace["latencyMs"],
            "spans": [],
        }
        
        for span_edge in trace.get("spans", {}).get("edges", []):
            span = span_edge["node"]
            # Parse attributes JSON if present
            attrs = span.get("attributes")
            if attrs and isinstance(attrs, str):
                try:
                    attrs = json.loads(attrs)
                except json.JSONDecodeError:
                    pass
            
            span_data = {
                "name": span["name"],
                "spanKind": span["spanKind"],
                "statusCode": span["statusCode"],
                "statusMessage": span.get("statusMessage"),
                "startTime": span["startTime"],
                "endTime": span["endTime"],
                "latencyMs": span["latencyMs"],
                "parentId": span["parentId"],
                "tokenCountTotal": span["tokenCountTotal"],
                "tokenCountPrompt": span["tokenCountPrompt"],
                "tokenCountCompletion": span["tokenCountCompletion"],
                "input": span.get("input", {}).get("value") if span.get("input") else None,
                "output": span.get("output", {}).get("value") if span.get("output") else None,
                "attributes": attrs,
            }
            trace_data["spans"].append(span_data)
        
        result["traces"].append(trace_data)
    
    return result


def main():
    parser = argparse.ArgumentParser(
        description="Fetch traces from Arize Phoenix for a specific session."
    )
    parser.add_argument(
        "session_id",
        help="The session ID to fetch traces for (output by elastic-package)",
    )
    parser.add_argument(
        "--output", "-o",
        help="Output file path (default: stdout)",
        default=None,
    )
    parser.add_argument(
        "--base-url",
        help="Phoenix base URL (default: http://localhost:6006)",
        default=os.environ.get("PHOENIX_BASE_URL", "http://localhost:6006"),
    )
    
    args = parser.parse_args()
    
    print(f"Fetching traces for session: {args.session_id}", file=sys.stderr)
    print(f"Phoenix URL: {args.base_url}", file=sys.stderr)
    
    try:
        result = fetch_traces(args.session_id, args.base_url)
    except Exception as e:
        print(f"Error fetching traces: {e}", file=sys.stderr)
        sys.exit(1)
    
    if not result:
        print(f"No session found for ID: {args.session_id}", file=sys.stderr)
        sys.exit(1)
    
    total_spans = sum(len(t["spans"]) for t in result["traces"])
    print(f"Found {result['numTraces']} trace(s) with {total_spans} span(s)", file=sys.stderr)
    
    # Output JSON
    output = json.dumps(result, indent=2, default=str)
    
    if args.output:
        with open(args.output, "w") as f:
            f.write(output)
        print(f"Traces written to: {args.output}", file=sys.stderr)
    else:
        print(output)


if __name__ == "__main__":
    main()
