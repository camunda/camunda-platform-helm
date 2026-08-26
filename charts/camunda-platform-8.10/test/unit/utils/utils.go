package utils

import "maps"

func MergeMaps(dst map[string]string, src map[string]string) map[string]string {
	maps.Insert(dst, maps.All(src))
	return dst
}
