package utils


func GetStringValue(data map[string]any, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}


func GetIntValue(data map[string]any, key string) int {
	if v, ok := data[key]; ok {
		switch value := v.(type) {
		case float64:
			return int(value)
		case int:
			return value
		case int64:
			return int(value)
		}
	}
	return 0
}
