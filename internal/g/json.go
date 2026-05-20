package g

import "encoding/json"

func MustParseJson[T any](data []byte) T {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}
