package wolfram

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/vedranvuk/locolm/internal/mcp"
)

func (self *Wolfram) registerWolframLLM(r mcp.Registry) {
	r.RegisterTool(
		"wolfram_llm",
		"Query Wolfram Alpha with LLM-optimized output (structured text, tables, image links). Recommended for most computational questions.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"input": {
					"type": "string",
					"description": "The query to compute. Natural language or mathematical expression (e.g. 'population of France', '2+2', 'integrate x^2 dx')."
				},
				"maxchars": {
					"type": "string",
					"description": "Maximum characters in the response (Wolfram default if omitted). Increase for complex queries, decrease for simple ones."
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
		self.wolframLLM,
	)
}

func (self *Wolfram) wolframLLM(args map[string]string) (string, error) {
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

	body, err := self.wolframGet("https://www.wolframalpha.com/api/v1/llm-api", params, 30)
	if err != nil {
		return "", err
	}

	// LLM API returns plain text directly — passthrough
	return string(body), nil
}
