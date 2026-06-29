package service

// IsAdmin reports whether the calling RequestContext (as injected by
// gateway via X-User-Roles) carries an admin-level role. document does not
// authenticate users itself; it only consumes the already-verified roles.
func (r RequestContext) IsAdmin() bool {
	for _, role := range r.Roles {
		if role == "admin" || role == "super_admin" {
			return true
		}
	}
	return false
}

// CanAccessReport reports whether the calling RequestContext may read or
// modify the given report: admins always can, standard users only for
// reports they created.
func (r RequestContext) CanAccessReport(report Report) bool {
	if r.IsAdmin() {
		return true
	}
	return r.UserID != "" && r.UserID == report.CreatorID
}
