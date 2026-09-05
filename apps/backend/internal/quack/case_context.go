package quack

import (
	"encoding/json"
	"strings"

	"github.com/quackdiscord/bot/internal/quack/model"
)

// validateCaseContextValues binds submitted values to the immutable template definitions and returns message links for shared capture.
func validateCaseContextValues(fields []model.CaseTemplateContextField, inputs []CaseContextValueInput) (string, []string, bool, error) {
	byKey := make(map[string]json.RawMessage, len(inputs))
	for _, input := range inputs {
		key := strings.ToLower(strings.TrimSpace(input.Key))
		if key == "" {
			return "", nil, false, validationCaseError("context value key is required")
		}
		if _, duplicate := byKey[key]; duplicate {
			return "", nil, false, validationCaseError("duplicate context value")
		}
		byKey[key] = input.Value
	}
	values := make([]CaseContextValueResponse, 0, len(fields))
	links := []string{}
	hasFallback := false
	for _, field := range fields {
		raw, provided := byKey[field.Key]
		if !provided || len(raw) == 0 || string(raw) == "null" {
			if field.Required {
				return "", nil, false, validationCaseError("required context value is missing: " + field.Key)
			}
			values = append(values, CaseContextValueResponse{Key: field.Key, Label: field.Label, FieldType: field.FieldType, Required: field.Required, Value: nil})
			delete(byKey, field.Key)
			continue
		}
		var value any
		switch field.FieldType {
		case model.ContextFieldShortText, model.ContextFieldLongText, model.ContextFieldMessageLink:
			var text string
			if json.Unmarshal(raw, &text) != nil {
				return "", nil, false, validationCaseError("context value has wrong type: " + field.Key)
			}
			text = strings.TrimSpace(text)
			limit := 4000
			if field.FieldType == model.ContextFieldShortText {
				limit = 500
			}
			if text == "" && field.Required {
				return "", nil, false, validationCaseError("required context value is empty: " + field.Key)
			}
			if len([]rune(text)) > limit {
				return "", nil, false, validationCaseError("context value is too long: " + field.Key)
			}
			value = text
			if field.FieldType == model.ContextFieldMessageLink && text != "" {
				links = append(links, text)
			} else if text != "" {
				hasFallback = true
			}
		case model.ContextFieldBoolean:
			var boolean bool
			if json.Unmarshal(raw, &boolean) != nil {
				return "", nil, false, validationCaseError("context value has wrong type: " + field.Key)
			}
			value = boolean
			hasFallback = true
		case model.ContextFieldNumber:
			var number json.Number
			decoder := json.NewDecoder(strings.NewReader(string(raw)))
			decoder.UseNumber()
			if decoder.Decode(&number) != nil {
				return "", nil, false, validationCaseError("context value has wrong type: " + field.Key)
			}
			if _, err := number.Float64(); err != nil {
				return "", nil, false, validationCaseError("context number is invalid: " + field.Key)
			}
			value = number
			hasFallback = true
		default:
			return "", nil, false, validationCaseError("context field type is invalid")
		}
		values = append(values, CaseContextValueResponse{Key: field.Key, Label: field.Label, FieldType: field.FieldType, Required: field.Required, Value: value})
		delete(byKey, field.Key)
	}
	if len(byKey) > 0 {
		return "", nil, false, validationCaseError("unknown context value")
	}
	body, err := json.Marshal(values)
	if err != nil {
		return "", nil, false, err
	}
	return string(body), links, hasFallback, nil
}

// parseCaseContextValues safely decodes the immutable member-visible context snapshot.
func parseCaseContextValues(body string) []CaseContextValueResponse {
	var values []CaseContextValueResponse
	if json.Unmarshal([]byte(body), &values) != nil {
		return []CaseContextValueResponse{}
	}
	return values
}
