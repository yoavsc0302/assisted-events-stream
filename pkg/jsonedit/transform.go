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
		res := value.Value()
		var item interface{}
		if res != nil {
			// unpack raw value
			var unpacked interface{}
			err = json.Unmarshal([]byte(value.String()), &unpacked)
			if err != nil {
				// when it can't be unmarshaled, it might be scalar?
				unpacked = value.String()
			}

			item, err = transformFn(unpacked)
		}

		if err != nil {
			continue
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
		return jsonBytes, fmt.Errorf("complexPath %s is not supported", complexPath)
	}
	parent := paths[0]
	child := strings.Trim(paths[1], ".")

	transformChildrenFn := func(unpacked interface{}) (interface{}, error) {
		parentList, ok := unpacked.([]interface{})
		if !ok {
			return unpacked, fmt.Errorf("node %s not a list", parent)
		}
		items := make([]interface{}, 0)
		for i := range parentList {
			var err error
			jsonBytes, err = json.Marshal(&parentList[i])
			if err != nil {
				return unpacked, err
			}
			jsonBytes, err = Transform(jsonBytes, []string{child}, transformChildFn)
			if err != nil {
				continue
			}
			var item interface{}
			err = json.Unmarshal(jsonBytes, &item)
			if err != nil {
				continue
			}
			items = append(items, item)
		}
		return items, nil
	}
	return Transform(jsonBytes, []string{parent}, transformChildrenFn)
}
