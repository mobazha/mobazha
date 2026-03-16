package mbzpb

// DisplayName returns the best available display name from the ID,
// preferring Name over Handle. Returns empty string if both are unset.
func (x *ID) DisplayName() string {
	if x == nil {
		return ""
	}
	if x.Name != "" {
		return x.Name
	}
	return x.Handle
}

// DisplayAvatar returns the tiny avatar hash from the ID, or empty string if nil.
func (x *ID) DisplayAvatar() string {
	if x == nil {
		return ""
	}
	return x.AvatarTiny
}
