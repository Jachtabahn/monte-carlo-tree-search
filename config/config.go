package config

func Int(key string) int {
	switch key {
	case "boardsize":
		return 5
	case "history_size":
		return 4
	}
	return -1
}

func Float(key string) float32 {
	switch key {
	case "komi":
		return 5.5
	}
	return -1.0
}
