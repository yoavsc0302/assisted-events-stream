package jsonedit

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Transforms from input `jsonBytes` each path in `paths`, applying `transformFn` to the unpacked `interface{}`.
func Transform(jsonBytes []byte, paths []string, transformFn func(unpacked interface{}) (interface{}, error)) ([]byte, error) {
	var err error
	for _, path := range paths {
		if isComplexPath(path) {
			jsonBytes, _ = transformFromComplexPath(jsonBytes, path, transformFn)
			continue
		}
		value := gjson.Get(string(jsonBytes), path)
		if !value.Exists() {
			continue
		}
		raw := value.String()

		// unpack raw value
		var unpacked interface{}
		err = json.Unmarshal([]byte(raw), &unpacked)
		if err != nil {
			// when it can't be codified, it might be scalar
			unpacked = raw
		}

		item, err := transformFn(unpacked)

		if err != nil {
			// handle error
		}

		// set changed value
		jsonString, err := sjson.Set(string(jsonBytes), path, item)
		if err != nil {
			continue
		}
		jsonBytes = []byte(jsonString)
	}
	return jsonBytes, nil
}

// Walks paths to apply `Transform` function when path is complex
func transformFromComplexPath(jsonBytes []byte, complexPath string, transformChildFn func(unpacked interface{}) (interface{}, error)) ([]byte, error) {
	paths := strings.Split(complexPath, "[*]")
	if len(paths) != 2 {
		return jsonBytes, fmt.Errorf("ComplexPath %s is not supported", complexPath)
	}
	parent := paths[0]
	child := strings.Trim(paths[1], ".")

	transformChildrenFn := func(unpacked interface{}) (interface{}, error) {
		parentList, ok := unpacked.([]interface{})
		if !ok {
			return unpacked, fmt.Errorf("Node %s not a list", parent)
		}
		items := make([]interface{}, 0)
		for _, v := range parentList {
			jsonBytes, err := json.Marshal(&v)
			if err != nil {
				return unpacked, err
			}
			jsonBytes, err = Transform(jsonBytes, []string{child}, transformChildFn)
			var item interface{}
			err = json.Unmarshal(jsonBytes, &item)
			if err != nil {
				// handle error
			}
			items = append(items, item)
		}
		return items, nil
	}
	return Transform(jsonBytes, []string{parent}, transformChildrenFn)
}
