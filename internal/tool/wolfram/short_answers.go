package wolfram

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/vedranvuk/locolm/internal/mcp"
)

func (self *Wolfram) registerWolframShort(r mcp.Registry) {
	r.RegisterTool(
		"wolfram_short",
		"Get a single short factual answer from Wolfram Alpha (e.g. 'capital of France', 'boiling point of water').",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"input": {
					"type": "string",
					"description": "The query to answer. Best for simple factual questions like 'What is the capital of France?', 'distance Earth Moon', or 'boiling point of water at sea level'."
				}
			},
			"required": ["input"]
		}`),
		self.wolframShort,
	)
}

func (self *Wolfram) wolframShort(args map[string]string) (string, error) {
	input, ok := args["input"]
	if !ok || input == "" {
		return "", fmt.Errorf("missing required argument: input")
	}

	params := url.Values{}
	params.Set("i", input)

	body, err := self.wolframGet("http://api.wolframalpha.com/v1/result", params, 15)
	if err != nil {
		return "", err
	}

	// Short Answers API returns a plain text string (may be empty)
	result := string(body)
	result = strings.TrimSpace(result)
	if result == "" {
		return "No short answer available for this query. Try wolfram_llm or wolfram_query for more detailed results.", nil
	}

	return result, nil
}
