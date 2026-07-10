package wolfram

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"github.com/vedranvuk/locolm/internal/mcp"
)

func (self *Wolfram) registerWolframQuery(r mcp.Registry) {
	r.RegisterTool(
		"wolfram_query",
		"Full Wolfram Alpha results with pod-level detail (math, science, conversions, data). Supports pod filtering, units, and location override.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"input": {
					"type": "string",
					"description": "The query to compute. Natural language (e.g. 'population of France'), math expressions (e.g. 'integrate x^2 dx from 0 to 1'), unit conversions, chemical formulas, date arithmetic, etc."
				},
				"format": {
					"type": "string",
					"description": "Output format per pod. One of: 'plaintext' (default), 'image' (returns Markdown image links), 'mathml' (raw MathML XML), 'minput' (Wolfram Language input), 'moutput' (Wolfram Language output)."
				},
				"include_pod": {
					"type": "string",
					"description": "Include only specific pods by ID. Comma-separated (e.g. 'Result,DecimalApproximation'). Use tools/list or see Wolfram docs for pod IDs."
				},
				"exclude_pod": {
					"type": "string",
					"description": "Exclude specific pods by ID. Comma-separated."
				},
				"pod_title": {
					"type": "string",
					"description": "Include only pods matching this title. Supports * wildcard (e.g. 'Basic Information*')."
				},
				"pod_index": {
					"type": "string",
					"description": "Include only pods by position index (1-based). Comma-separated (e.g. '1,2,3')."
				},
				"scanner": {
					"type": "string",
					"description": "Include only pods from specific scanners (e.g. 'Data', 'Numeric', 'MathematicalFunctionData'). Comma-separated."
				},
				"location": {
					"type": "string",
					"description": "Override query location for location-dependent results (e.g. 'Boston, MA', 'Tokyo', 'London')."
				},
				"units": {
					"type": "string",
					"description": "Unit system for results: 'metric' or 'nonmetric' (US customary).",
					"enum": ["metric", "nonmetric"]
				},
				"width": {
					"type": "string",
					"description": "Approximate width for text/table pods in pixels (default 500). Does not affect plots."
				},
				"timeout": {
					"type": "string",
					"description": "Max seconds for the query (default 30). Increase for heavy computations like large integrals or complex data queries."
				},
				"assumption": {
					"type": "string",
					"description": "Apply an assumption token from a previous query's assumptions to disambiguate the query."
				}
			},
			"required": ["input"]
		}`),
		self.wolframQuery,
	)
}

func (self *Wolfram) wolframQuery(args map[string]string) (string, error) {
	input, ok := args["input"]
	if !ok || input == "" {
		return "", fmt.Errorf("missing required argument: input")
	}

	params := url.Values{}
	params.Set("input", input)
	params.Set("output", "xml")

	if v := args["format"]; v != "" {
		params.Set("format", v)
	}
	if v := args["include_pod"]; v != "" {
		for _, id := range strings.Split(v, ",") {
			params.Add("includepodid", strings.TrimSpace(id))
		}
	}
	if v := args["exclude_pod"]; v != "" {
		for _, id := range strings.Split(v, ",") {
			params.Add("excludepodid", strings.TrimSpace(id))
		}
	}
	if v := args["pod_title"]; v != "" {
		params.Set("podtitle", v)
	}
	if v := args["pod_index"]; v != "" {
		params.Set("podindex", v)
	}
	if v := args["scanner"]; v != "" {
		params.Set("scanner", v)
	}
	if v := args["location"]; v != "" {
		params.Set("location", v)
	}
	if v := args["units"]; v != "" {
		params.Set("units", v)
	}
	if v := args["width"]; v != "" {
		params.Set("width", v)
	}
	if v := args["timeout"]; v != "" {
		params.Set("totaltimeout", v)
	}
	if v := args["assumption"]; v != "" {
		params.Set("assumption", v)
	}

	timeoutSec := parseIntOr(args["timeout"], 30)
	body, err := self.wolframGet("http://api.wolframalpha.com/v2/query", params, timeoutSec)
	if err != nil {
		return "", err
	}

	var result QueryResult
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Wolfram API response: %w", err)
	}

	if result.Error == "true" {
		return "", fmt.Errorf("Wolfram API returned an error (possible invalid AppID or malformed query)")
	}

	if result.Success == "false" {
		// Query could not be interpreted — provide helpful feedback
		var sb strings.Builder
		sb.WriteString("Wolfram Alpha could not interpret this query.\n")
		sb.WriteString(formatDidYouMeans(result.DidYouMeans))
		sb.WriteString(formatWarnings(result.Warnings))
		return sb.String(), nil
	}

	// Build output
	var sb strings.Builder
	fmt.Fprintf(&sb, "Query: %s\n", input)
	if result.DataTypes != "" {
		fmt.Fprintf(&sb, "Data types: %s\n", result.DataTypes)
	}
	if result.Timing != "" {
		fmt.Fprintf(&sb, "Timing: %ss\n", result.Timing)
	}
	fmt.Fprintf(&sb, "Pods: %.0f\n\n", result.NumPods)

	for _, pod := range result.Pods {
		if pod.Error == "true" {
			fmt.Fprintf(&sb, "--- %s [error] ---\n", pod.Title)
			continue
		}
		sb.WriteString(formatPod(pod, args["format"]))
	}

	// Append metadata
	sb.WriteString(formatAssumptions(result.Assumptions))
	sb.WriteString(formatWarnings(result.Warnings))
	sb.WriteString(formatSources(result.Sources))

	if result.Recalculate != "" {
		fmt.Fprintf(&sb, "\nMore results available: %s\n", result.Recalculate)
	}

	return sb.String(), nil
}
