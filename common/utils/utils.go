package utils

import (
	"encoding/json"
	"fmt"
)

// PrettyJSON returns a pretty json format string from any data
func PrettyJSON(data interface{}) string {
	json, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Sprintf("PrettJSON %#v failed: %s", data, err)
	}

	return string(json)
}

