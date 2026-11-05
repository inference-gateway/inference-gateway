package systemone

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

const (
	testModel     = "nimble"
	testInstr     = "Is this spam?"
	testEvidence  = `"Buy now!!!"`
	testItemsBody = `[{"type":"message","role":"user","content":[{"type":"input_text","text":"Buy now!!!"}]}]`
)

func TestRequest(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "predicate becomes noul over a string state",
			body:     `{"model":"ollama/nimble","input":` + testEvidence + `,"questions":[{"type":"predicate","name":"is_spam","instructions":"` + testInstr + `"}]}`,
			expected: `{"model":"nimble","state":` + testEvidence + `,"questions":{"is_spam":{"type":"noul","instructions":"` + testInstr + `"}}}`,
		},
		{
			name:     "choice carries value to description criteria and an item list state",
			body:     `{"model":"nimble","input":` + testItemsBody + `,"questions":[{"type":"choice","name":"topic","instructions":"Pick","choices":[{"value":"billing","description":"Payments"},{"value":"tech","description":"Bugs"}]}]}`,
			expected: `{"model":"nimble","state":` + testItemsBody + `,"questions":{"topic":{"type":"choice","instructions":"Pick","criteria":{"billing":"Payments","tech":"Bugs"}}}}`,
		},
		{
			name:     "score carries ordered level criteria, label when description is missing",
			body:     `{"model":"nimble","input":"x","questions":[{"type":"score","name":"tone","instructions":"Rate","levels":[{"label":"rude"},{"label":"ok","description":"Neutral"},{"label":"kind"}]}]}`,
			expected: `{"model":"nimble","state":"x","questions":{"tone":{"type":"score","instructions":"Rate","criteria":["rude","Neutral","kind"]}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Request(testModel, []byte(tt.body))
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(out))
		})
	}
}

func TestRequestErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		msg  string
	}{
		{"no questions", `{"model":"nimble","input":"x","questions":[]}`, "at least one question"},
		{"missing name", `{"model":"nimble","input":"x","questions":[{"type":"predicate","instructions":"?"}]}`, "needs a 'name'"},
		{"choice without choices", `{"model":"nimble","input":"x","questions":[{"type":"choice","name":"c","instructions":"?"}]}`, "needs 'choices'"},
		{"score without levels", `{"model":"nimble","input":"x","questions":[{"type":"score","name":"s","instructions":"?"}]}`, "needs 'levels'"},
		{"unknown type", `{"model":"nimble","input":"x","questions":[{"type":"rank","name":"r","instructions":"?"}]}`, "unsupported type"},
		{"duplicate question name", `{"model":"nimble","input":"x","questions":[{"type":"predicate","name":"q","instructions":"?"},{"type":"predicate","name":"q","instructions":"?"}]}`, "unique 'name'"},
		{"duplicate choice value", `{"model":"nimble","input":"x","questions":[{"type":"choice","name":"c","instructions":"?","choices":[{"value":"a","description":"A"},{"value":"a","description":"B"}]}]}`, "unique 'value'"},
		{"duplicate choice value when both are missing", `{"model":"nimble","input":"x","questions":[{"type":"choice","name":"c","instructions":"?","choices":[{"description":"A"},{"description":"B"}]}]}`, "unique 'value'"},
		{"bad json", `{nope`, "failed to decode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Request(testModel, []byte(tt.body))
			require.ErrorContains(t, err, tt.msg)
		})
	}
}

func TestResponse(t *testing.T) {
	body := `{"model":"nimble","answers":{"topic":{"type":"choice","choice":"billing","probabilities":{"billing":0.9,"tech":0.1},"confidence":0.8},"is_spam":{"type":"noul","noul":0.97}},"usage":{"input_tokens":12,"output_tokens":0}}`
	out, err := Response([]byte(body))
	require.NoError(t, err)
	assert.JSONEq(t, `{"answers":[{"name":"is_spam","type":"predicate","probability":0.97},{"name":"topic","type":"choice","choice":"billing","probabilities":{"billing":0.9,"tech":0.1},"confidence":0.8}],"usage":{"input_tokens":12}}`, string(out))

	_, err = Response([]byte(`{nope`))
	require.ErrorContains(t, err, "failed to decode upstream response")
}
