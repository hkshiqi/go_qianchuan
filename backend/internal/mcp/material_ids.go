package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func stringIDList(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		if typed, typedOK := value.([]string); typedOK {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		} else {
			return nil, errors.New("ID list must be an array")
		}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(fmt.Sprint(item))
		if id == "" {
			continue
		}
		if strings.Contains(id, ".") {
			parsed, err := strconv.ParseFloat(id, 64)
			if err != nil || parsed != float64(int64(parsed)) {
				return nil, fmt.Errorf("invalid ID: %s", id)
			}
			id = strconv.FormatInt(int64(parsed), 10)
		}
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			return nil, fmt.Errorf("invalid ID: %s", id)
		}
		result = append(result, id)
	}
	return result, nil
}

func stringIDListArg(args map[string]any, key string) ([]string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	return stringIDList(value)
}

func materialIDsFromJSON(value any) ([]uint64, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var stringsList []json.Number
	if err := json.Unmarshal(raw, &stringsList); err != nil {
		return nil, err
	}
	result := make([]uint64, 0, len(stringsList))
	for _, number := range stringsList {
		parsed, err := strconv.ParseUint(number.String(), 10, 64)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}
