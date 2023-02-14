package process

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/openshift-assisted/assisted-events-streams/pkg/jsonedit"
)

// Anonymize sensitive data
func anonymizeJson(jsonBytes []byte, paths map[string]string) ([]byte, error) {
	keys := []string{}
	for k, _ := range paths {
		keys = append(keys, k)
	}
	hash := func(unpacked interface{}) (interface{}, error) {
		if sensitive, ok := unpacked.(string); ok {
			hash := md5.New()
			hash.Write([]byte(sensitive))
			hashBytes := hash.Sum(nil)
			return hex.EncodeToString(hashBytes[:]), nil
		}
		return unpacked, nil
	}
	jsonBytes, err := jsonedit.Transform(jsonBytes, keys, hash)
	if err != nil {
		return jsonBytes, fmt.Errorf("failed to anonymize")
	}
	return jsonedit.Rename(jsonBytes, paths)
}
