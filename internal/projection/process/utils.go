package process

import (
	"fmt"
)

func GetValueFromPayload(key string, payload interface{}) (string, error) {
	if payload, ok := (payload).(map[string]interface{}); ok {
		if v, ok := payload[key]; ok {
			if value, ok := v.(string); ok {
				return value, nil
			}
		}
	}
	return "", NewMalformedEventError(fmt.Sprintf("Error retrieving key %s from payload (%v)", key, payload))
}
