package util

import "os"

// Getenv Getenv
func Getenv(key string, defaultVal string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}
	return defaultVal

}
