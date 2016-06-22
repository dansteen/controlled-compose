package control

// GetIndex will find the index of a value in a slice. if no matching value is found it returns a -1
func GetIndex(slice []string, value string) int {
	for index, item := range slice {
		if value == item {
			return index
		}
	}
	return -1
}
