package requests

import "encoding/json"

// 独立的泛型函数解析数据
func ParseJSON[T any](obj *Response) (T, error) {
	var result T
	err := json.Unmarshal(obj.Content(), &result)
	return result, err
}
