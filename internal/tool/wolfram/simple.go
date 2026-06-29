package wolfram

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/vedranvuk/locolm/internal/mcp"
)

func registerWolframImage() {
	mcp.RegisterTool(
		"wolfram_image",
		"Query Wolfram Alpha and get a rendered image of the full result page. Returns a Markdown image link showing the visual Wolfram Alpha output.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"input": {
					"type": "string",
					"description": "The query to compute and render as an image."
				},
				"width": {
					"type": "string",
					"description": "Image width in pixels (default 500)."
				},
				"font_size": {
					"type": "string",
					"description": "Font size for the rendering (e.g. '12', '14')."
				},
				"layout": {
					"type": "string",
					"description": "Layout style: 'label' or 'flow'."
				},
				"units": {
					"type": "string",
					"description": "Unit system: 'metric' or 'nonmetric'.",
					"enum": ["metric", "nonmetric"]
				},
				"location": {
					"type": "string",
					"description": "Override query location."
				},
				"exclude_pod": {
					"type": "string",
					"description": "Exclude specific pods by ID from the rendering. Comma-separated."
				},
				"include_pod": {
					"type": "string",
					"description": "Include only specific pods by ID. Comma-separated."
				}
			},
			"required": ["input"]
		}`),
		wolframImage,
	)
}

func wolframImage(args map[string]string) (string, error) {
	input, ok := args["input"]
	if !ok || input == "" {
		return "", fmt.Errorf("missing required argument: input")
	}

	params := url.Values{}
	params.Set("i", input)

	if v := args["width"]; v != "" {
		params.Set("width", v)
	}
	if v := args["font_size"]; v != "" {
		params.Set("fontsize", v)
	}
	if v := args["layout"]; v != "" {
		params.Set("layout", v)
	}
	if v := args["units"]; v != "" {
		params.Set("units", v)
	}
	if v := args["location"]; v != "" {
		params.Set("location", v)
	}
	if v := args["exclude_pod"]; v != "" {
		for _, id := range strings.Split(v, ",") {
			params.Add("excludepodid", strings.TrimSpace(id))
		}
	}
	if v := args["include_pod"]; v != "" {
		for _, id := range strings.Split(v, ",") {
			params.Add("includepodid", strings.TrimSpace(id))
		}
	}

	// Simple API returns raw image bytes (PNG or GIF), not a URL.
	// We make the HTTP call and return the full URL for the client to display.
	imageURL, contentType, err := wolframGetImage("http://api.wolframalpha.com/v1/simple", params, 30)
	if err != nil {
		return "", err
	}
	_ = contentType // could be used for logging

	return fmt.Sprintf("![Wolfram Alpha result for '%s'](%s)", input, imageURL), nil
}
