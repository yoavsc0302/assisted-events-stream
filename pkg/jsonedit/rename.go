package jsonedit

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Rename defined values to be renamed. Accepts map[string]string with key as srcPath and value
// for dstPath
func Rename(jsonBytes []byte, paths map[string]string) ([]byte, error) {
	for srcPath, dstPath := range paths {
		if isComplexPath(srcPath) {
			srcPathParts := strings.Split(srcPath, "[*]")
			dstPathParts := strings.Split(dstPath, "[*]")
			if len(srcPathParts) != 2 || len(dstPathParts) != 2 {
				// not supported
				continue
			}

			srcParent := srcPathParts[0]
			srcChild := strings.Trim(srcPathParts[1], ".")
			dstParent := dstPathParts[0]
			dstChild := strings.Trim(dstPathParts[1], ".")

			if srcParent != dstParent {
				// not supported
				continue
			}
			value := gjson.Get(string(jsonBytes), srcParent)
			if !value.Exists() {
				continue
			}
			raw := value.String()

			var unpacked interface{}
			err := json.Unmarshal([]byte(raw), &unpacked)
			if err != nil {
				continue
			}

			renamePaths := map[string]string{}
			renamePaths[srcChild] = dstChild
			parentList := unpacked.([]interface{})
			items := []interface{}{}

			for _, child := range parentList {
				var childBytes []byte
				childBytes, err = json.Marshal(child)
				if err != nil {
					continue
				}
				var renamedBytes []byte
				renamedBytes, err = Rename(childBytes, renamePaths)
				if err != nil {
					continue
				}
				var item interface{}
				err = json.Unmarshal(renamedBytes, &item)
				if err != nil {
					// could be scalar
					item = raw
				}

				items = append(items, item)
			}

			jsonString, err := sjson.Set(string(jsonBytes), dstParent, items)
			if err != nil {
				continue
			}

			jsonBytes = []byte(jsonString)
			continue
		}
		value := gjson.Get(string(jsonBytes), srcPath)
		if !value.Exists() {
			continue
		}
		raw := value.String()
		var item interface{}
		err := json.Unmarshal([]byte(raw), &item)
		if err != nil {
			// could be scalar
			item = raw
		}

		jsonString, err := sjson.Set(string(jsonBytes), dstPath, item)
		if err != nil {
			continue
		}

		jsonString, err = sjson.Delete(jsonString, srcPath)
		if err != nil {
			return jsonBytes, err
		}
		jsonBytes = []byte(jsonString)
	}
	return jsonBytes, nil
}
