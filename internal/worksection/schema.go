package worksection

// Schema returns the local parameter metadata for a known action.
func Schema(action string) (Action, bool) {
	return LookupAction(action)
}
