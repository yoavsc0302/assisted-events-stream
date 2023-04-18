package jsonedit

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/sjson"
)

// Deletes defined values to delete
func Delete(jsonBytes []byte, paths []string) ([]byte, error) {
	var err error
	for _, path := range paths {
		if isComplexPath(path) {
			jsonBytes, err = deleteFromComplexPath(jsonBytes, path)
			if err != nil {
				return jsonBytes, err
			}
			continue
		}
		strEventJson, err := sjson.Delete(string(jsonBytes), path)
		if err != nil {
			return jsonBytes, err
		}
		jsonBytes = []byte(strEventJson)
	}
	return jsonBytes, nil
}

// Deletes defined values to delete
func deleteFromComplexPath(jsonBytes []byte, complexPath string) ([]byte, error) {
	paths := strings.Split(complexPath, "[*]")
	if len(paths) != 2 {
		return jsonBytes, fmt.Errorf("complexPath %s is not supported", complexPath)
	}
	parent := paths[0]
	child := strings.Trim(paths[1], ".")
	deleteChild := func(unpacked interface{}) (interface{}, error) {
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
			jsonString, err := sjson.Delete(string(jsonBytes), child)
			if err != nil {
				return unpacked, err
			}
			var item interface{}
			err = json.Unmarshal([]byte(jsonString), &item)
			if err != nil {
				return unpacked, err
			}
			items = append(items, item)
		}
		return items, nil
	}
	return Transform(jsonBytes, []string{parent}, deleteChild)
}
