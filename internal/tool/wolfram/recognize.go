package wolfram

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"github.com/vedranvuk/locolm/internal/mcp"
)

func (self *Wolfram) registerWolframRecognize(r mcp.Registry) {
	r.RegisterTool(
		"wolfram_recognize",
		"Check whether Wolfram Alpha can compute a query (triage before a full query). May be unavailable on some API tiers.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"input": {
					"type": "string",
					"description": "The query to classify."
				}
			},
			"required": ["input"]
		}`),
		self.wolframRecognize,
	)
}

func (self *Wolfram) wolframRecognize(args map[string]string) (string, error) {
	input, ok := args["input"]
	if !ok || input == "" {
		return "", fmt.Errorf("missing required argument: input")
	}

	params := url.Values{}
	params.Set("i", input)
	params.Set("mode", "Default")

	body, err := self.wolframGet("https://www.wolframalpha.com/queryrecognizer/query.jsp", params, 10)
	if err != nil {
		return "", err
	}

	// The recognizer returns XML with a different structure
	type QueryRecognizerResult struct {
		XMLName            xml.Name `xml:"queryrecognizer"`
		Version            string   `xml:"version,attr"`
		SpellingCorrection string   `xml:"spellingcorrection,attr"`
		BuildNumber        string   `xml:"buildnumber,attr"`
		Query              struct {
			XMLName  xml.Name `xml:"query"`
			Input    string   `xml:"i,attr"`
			Accepted string   `xml:"accepted,attr"`
			Timing   string   `xml:"timing,attr"`
			Domain   string   `xml:"domain,attr"`
			Score    string   `xml:"resultsignificancescore,attr"`
		} `xml:"query"`
	}

	var result QueryRecognizerResult
	if err := xml.Unmarshal(body, &result); err != nil {
		// Fallback: return raw response
		return fmt.Sprintf("Raw classification response:\n%s", truncate(string(body), 1000)), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Query: %s\n", input)
	fmt.Fprintf(&sb, "Accepted: %s\n", result.Query.Accepted)
	fmt.Fprintf(&sb, "Domain: %s\n", result.Query.Domain)
	fmt.Fprintf(&sb, "Score: %s\n", result.Query.Score)
	fmt.Fprintf(&sb, "Timing: %sms\n", result.Query.Timing)

	return sb.String(), nil
}
