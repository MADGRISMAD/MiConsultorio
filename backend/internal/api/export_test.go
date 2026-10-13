package api

// AITaskKeys lists the keys of the AI task registry for the external tests.
func AITaskKeys() []string {
	keys := make([]string, 0, len(aiTasks))
	for _, t := range aiTasks {
		keys = append(keys, t.Key)
	}
	return keys
}
