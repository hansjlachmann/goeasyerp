package session

import "sync/atomic"

// securityVersion counts the changes to users, roles and permissions in this server: a cached
// session made before the latest change is checked again before its next request
var securityVersion atomic.Int64

// SecurityChanged tells the sessions that a user, role or permission changed (the triggers of
// User, User_Member, User_Role and Permission)
func SecurityChanged() { securityVersion.Add(1) }

// SecurityVersion is the number of such changes so far
func SecurityVersion() int64 { return securityVersion.Load() }
