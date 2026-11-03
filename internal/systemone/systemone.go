package systemone

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// System One is the decision wire format Ollama and llama.cpp serve at POST
// /v1/systemone: shared evidence in state, questions keyed by name, answers
// keyed by name. The OpenAI Decisions API carries the same three question
// kinds under other names, so both directions are field renames.
const (
	typeNoul   = "noul"
	typeChoice = "choice"
	typeScore  = "score"
)

type question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type request struct {
	Model     string              `json:"model"`
	State     json.RawMessage     `json:"state"`
	Questions map[string]question `json:"questions"`
}

type decisionRequest struct {
	Input     json.RawMessage          `json:"input"`
	Questions []types.DecisionQuestion `json:"questions"`
}

// Request rewrites a CreateDecisionRequest body into the System One request
// for model. The input rides as the state untouched (a string or the item
// list, both valid state), predicate becomes noul, choices become the
// criteria map and levels the ordered criteria list.
func Request(model string, body []byte) ([]byte, error) {
	var req decisionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to decode request: %w", err)
	}
	if len(req.Questions) == 0 {
		return nil, fmt.Errorf("the 'questions' field must list at least one question")
	}
	out := request{Model: model, State: req.Input, Questions: make(map[string]question, len(req.Questions))}
	for _, q := range req.Questions {
		if q.Name == "" {
			return nil, fmt.Errorf("every question needs a 'name'")
		}
		translated, err := translateQuestion(q)
		if err != nil {
			return nil, err
		}
		out.Questions[q.Name] = translated
	}
	return json.Marshal(out)
}

func translateQuestion(q types.DecisionQuestion) (question, error) {
	switch q.Type {
	case types.Predicate:
		return question{Type: typeNoul, Instructions: q.Instructions}, nil
	case types.Choice:
		if q.Choices == nil || len(*q.Choices) == 0 {
			return question{}, fmt.Errorf("choice question %q needs 'choices'", q.Name)
		}
		criteria := make(map[string]string, len(*q.Choices))
		for _, c := range *q.Choices {
			criteria[deref(c.Value)] = deref(c.Description)
		}
		return question{Type: typeChoice, Instructions: q.Instructions, Criteria: criteria}, nil
	case types.Score:
		if q.Levels == nil || len(*q.Levels) == 0 {
			return question{}, fmt.Errorf("score question %q needs 'levels'", q.Name)
		}
		criteria := make([]string, 0, len(*q.Levels))
		for _, l := range *q.Levels {
			criteria = append(criteria, firstNonEmpty(deref(l.Description), deref(l.Label)))
		}
		return question{Type: typeScore, Instructions: q.Instructions, Criteria: criteria}, nil
	}
	return question{}, fmt.Errorf("question %q has unsupported type %q", q.Name, q.Type)
}

type response struct {
	Answers map[string]map[string]any `json:"answers"`
	Usage   struct {
		InputTokens int64 `json:"input_tokens"`
	} `json:"usage"`
}

type decision struct {
	Answers []map[string]any `json:"answers"`
	Usage   struct {
		InputTokens int64 `json:"input_tokens"`
	} `json:"usage"`
}

// Response rewrites a System One response into the Decision shape: the
// answers object becomes a list carrying each name, a noul answer becomes a
// predicate with its probability, and the usage keeps the input tokens.
// Answers are ordered by name since the upstream object has no order.
func Response(body []byte) ([]byte, error) {
	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode upstream response: %w", err)
	}
	out := decision{Answers: make([]map[string]any, 0, len(resp.Answers))}
	out.Usage.InputTokens = resp.Usage.InputTokens
	for _, name := range slices.Sorted(maps.Keys(resp.Answers)) {
		out.Answers = append(out.Answers, translateAnswer(name, resp.Answers[name]))
	}
	return json.Marshal(out)
}

func translateAnswer(name string, answer map[string]any) map[string]any {
	translated := make(map[string]any, len(answer)+1)
	for k, v := range answer {
		translated[k] = v
	}
	translated["name"] = name
	if answer["type"] == typeNoul {
		translated["type"] = string(types.Predicate)
		translated["probability"] = answer[typeNoul]
		delete(translated, typeNoul)
	}
	return translated
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
