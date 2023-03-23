package process

import (
	"fmt"

	"github.com/openshift-assisted/assisted-events-streams/pkg/jsonedit"
)

// Unpacks defined values to unpack
func unpackJson(jsonBytes []byte, paths []string) ([]byte, error) {
	unpack := func(unpacked interface{}) (interface{}, error) {
		return unpacked, nil
	}
	return jsonedit.Transform(jsonBytes, paths, unpack)
}

// Transforms defined values from map to list (removing the first level key)
func mapToListJsonDropKey(jsonBytes []byte, paths []string) ([]byte, error) {
	mapToList := func(unpacked interface{}) (interface{}, error) {
		srcMap, ok := unpacked.(map[string]interface{})
		if !ok {
			return unpacked, fmt.Errorf("Field is not a map")
		}
		var dstList []interface{}
		for _, v := range srcMap {
			dstList = append(dstList, v)
		}
		return dstList, nil
	}
	return jsonedit.Transform(jsonBytes, paths, mapToList)
}

// Transforms defined values from map to list.
// Keeps first level key as "id" if item value is map-like, otherwise
// creates each item to be a map like: {"key": key, "value": value}
func mapToListJson(jsonBytes []byte, paths []string) ([]byte, error) {
	mapToList := func(unpacked interface{}) (interface{}, error) {
		srcMap, ok := unpacked.(map[string]interface{})
		if !ok {
			return unpacked, fmt.Errorf("Field is not a map")
		}
		var dstList []interface{}
		for k, v := range srcMap {
			if item, ok := v.(map[string]interface{}); ok {
				item["id"] = k
				dstList = append(dstList, item)
			} else {
				item := map[string]interface{}{}
				item["key"] = k
				item["value"] = v
				dstList = append(dstList, item)
			}
		}
		return dstList, nil
	}
	return jsonedit.Transform(jsonBytes, paths, mapToList)
}
