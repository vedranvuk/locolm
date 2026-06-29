package wolfram

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/vedranvuk/locolm/internal/mcp"
)

func registerWolframLLM() {
	mcp.RegisterTool(
		"wolfram_llm",
		"Query Wolfram Alpha and get results optimized for LLM consumption. Returns structured text tables, image URLs, and a link to the full results page. This is the recommended tool for most queries.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"input": {
					"type": "string",
					"description": "The query to compute. Natural language or mathematical expression (e.g. 'population of France', '2+2', 'integrate x^2 dx')."
				},
				"maxchars": {
					"type": "string",
					"description": "Maximum characters in response (default 6800). Increase for complex queries, decrease for simple ones."
				},
				"location": {
					"type": "string",
					"description": "Override query location for location-dependent results (e.g. 'Boston, MA', 'Tokyo')."
				},
				"units": {
					"type": "string",
					"description": "Unit system: 'metric' or 'nonmetric' (US customary).",
					"enum": ["metric", "nonmetric"]
				},
				"assumption": {
					"type": "string",
					"description": "Apply an assumption token from a previous query's assumptions to disambiguate the query."
				}
			},
			"required": ["input"]
		}`),
		wolframLLM,
	)
}

func wolframLLM(args map[string]string) (string, error) {
	input, ok := args["input"]
	if !ok || input == "" {
		return "", fmt.Errorf("missing required argument: input")
	}

	params := url.Values{}
	params.Set("input", input)

	if v := args["maxchars"]; v != "" {
		params.Set("maxchars", v)
	}
	if v := args["location"]; v != "" {
		params.Set("location", v)
	}
	if v := args["units"]; v != "" {
		params.Set("units", v)
	}
	if v := args["assumption"]; v != "" {
		params.Set("assumption", v)
	}

	body, err := wolframGet("https://www.wolframalpha.com/api/v1/llm-api", params, 30)
	if err != nil {
		return "", err
	}

	// LLM API returns plain text directly — passthrough
	return string(body), nil
}
