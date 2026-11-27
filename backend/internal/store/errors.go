package store

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound means the row does not exist or is soft deleted.
	ErrNotFound = errors.New("not found")
	// ErrDuplicateEntry wraps a unique-constraint violation together with the driver error.
	ErrDuplicateEntry = errors.New("duplicate entry")
	// ErrProtected means the row is seeded or otherwise owned by the system.
	ErrProtected = errors.New("protected")
	// ErrInUse means another row still references this one.
	ErrInUse = errors.New("in use")
	// ErrHasChildren means the row has children and cannot be deleted.
	ErrHasChildren = errors.New("has children")
	// ErrHasDescendant means the move would place a node below itself.
	ErrHasDescendant = errors.New("descendant")
	// ErrReadonly means the row is exposed for reading only.
	ErrReadonly = errors.New("readonly")
	// ErrNotEditable means the row exists but may not be modified.
	ErrNotEditable = errors.New("not editable")
	// ErrCannotPublish means the notice is not in a publishable state.
	ErrCannotPublish = errors.New("cannot publish")
	// ErrCannotRevoke means the notice must be revoked before it can be deleted.
	ErrCannotRevoke = errors.New("must be revoked first")
	// ErrNoticeRevokeExpired means the revoke window after publishing has elapsed.
	ErrNoticeRevokeExpired = errors.New("revoke window expired")
	// ErrDefaultOrInUse means a storage config is the default one or still referenced by files.
	ErrDefaultOrInUse = errors.New("default or in use")
	// ErrUserProtected means the user is missing or is the built-in administrator.
	ErrUserProtected = errors.New("user not found or protected")
	// ErrRoleProtected means the role is missing or is the built-in super administrator.
	ErrRoleProtected = errors.New("role not found or protected")
	// ErrMenuHasChildren means a menu with children cannot be deleted.
	ErrMenuHasChildren = errors.New("menu has children")
	// ErrMenuCycle means a menu was set as its own parent.
	ErrMenuCycle = errors.New("menu cannot be its own parent")
	// ErrMenuDescendant means a menu was moved below one of its own descendants.
	ErrMenuDescendant = errors.New("menu cannot be moved below its descendant")
	// ErrMenuCodeTaken means a declared code collides with a row the declaring
	// owner may not take over: a permission code that a page already uses, or a
	// menu code that belongs to another plugin. The sync refuses either one
	// instead of silently skipping it.
	ErrMenuCodeTaken = errors.New("menu code already taken by another owner")
	// ErrMenuParentMissing means a declared menu points at a parent code that
	// does not exist, which is almost always a typo.
	ErrMenuParentMissing = errors.New("declared menu parent does not exist")
	// ErrMenuButtonManaged means the caller tried to change a field the
	// startup sync owns.
	ErrMenuButtonManaged = errors.New("permission buttons are managed by the route table")
	// ErrMenuParentInvalid means a button was given a parent that cannot own
	// one (a button, or a menu that does not exist).
	ErrMenuParentInvalid = errors.New("button parent must be an existing directory or page")
	// ErrDictHasItems means a dictionary type is seeded or still has items.
	ErrDictHasItems = errors.New("dictionary type is protected or has items")
	// ErrDictCodeLocked means a system dictionary's code cannot change: item
	// entity keys (dict.<code>.<value>) and the seed tenant template both key
	// off the code, so renaming it would silently orphan every reference.
	ErrDictCodeLocked = errors.New("system dictionary code is locked")
	// ErrDictItemInUse means the item value is still referenced by business
	// rows registered through RegisterDictReference, so disabling it would
	// leave historical data without a label and deleting it would orphan them.
	ErrDictItemInUse = errors.New("dictionary item is still referenced")
	// ErrDepartmentProtected means the row is the root department, which the
	// tree is anchored on and therefore cannot be moved or deleted.
	ErrDepartmentProtected = errors.New("root department is protected")
	// ErrDepartmentCycle means a department was set as its own parent or moved
	// below one of its own descendants.
	ErrDepartmentCycle = errors.New("department hierarchy would form a cycle")
	// ErrDepartmentInUse means the department still has children or users.
	ErrDepartmentInUse = errors.New("department still has children or users")
	// ErrDepartmentParentDisabled means the target parent is disabled, and an
	// active department may not hang under an inactive one.
	ErrDepartmentParentDisabled = errors.New("parent department is disabled")
	// ErrPositionInUse means the position is still assigned to a user.
	ErrPositionInUse = errors.New("position is still assigned to a user")
	// ErrTokenReused means a refresh token that was already rotated was
	// presented again. The caller must revoke the whole rotation family: this is
	// the signature of a stolen token being replayed.
	ErrTokenReused = errors.New("refresh token was already used")
	// ErrTokenExpired means an opaque token is past its expiry.
	ErrTokenExpired = errors.New("token has expired")
	// ErrIPNotAllowed means the caller's address is outside the token's
	// allowlist.
	ErrIPNotAllowed = errors.New("client IP is not allowed for this token")
	// ErrOAuthSubjectMissing means the provider returned no subject id, so no
	// identity can be resolved.
	ErrOAuthSubjectMissing = errors.New("external user id is missing")
	// ErrOAuthBindingRequired means no local account matched the external
	// identity and the provider is not allowed to auto-register.
	ErrOAuthBindingRequired = errors.New("external account requires manual binding")
	// ErrFileStateConflict means the file exists but is not in the state the
	// operation requires (for example, purging a file that is still ready).
	ErrFileStateConflict = errors.New("file is not in the required state")
	// ErrMigrationBusy is returned when another process holds the advisory
	// lock that serialises plugin schema changes.
	ErrMigrationBusy = errors.New("migration is busy")
	// ErrQuotaExceeded means a tenant quota row is missing or the increment
	// would pass its limit.
	ErrQuotaExceeded = errors.New("tenant quota exceeded")
	// ErrInvalidSubscriptionTransition means the requested status is not a
	// legal move from the tenant's open subscription.
	ErrInvalidSubscriptionTransition = errors.New("invalid subscription transition")
	// ErrPurgeNotCancelable means the tenant is not waiting on a future purge.
	ErrPurgeNotCancelable = errors.New("purge cannot be cancelled")
	// ErrInvalidPurgeWindow means the requested purge delay is out of range.
	ErrInvalidPurgeWindow = errors.New("purge window is invalid")
	// ErrInviteInvalid means an invitation token is unknown, already used,
	// revoked or expired.
	ErrInviteInvalid = errors.New("invitation is invalid or expired")
	// ErrInviteEmailMismatch means the signed-in account does not own the
	// address the invitation was sent to.
	ErrInviteEmailMismatch = errors.New("invitation belongs to another email address")
	// ErrLastTenantAdmin means the change would leave the tenant with no active
	// administrator, which cannot be repaired from inside the tenant.
	ErrLastTenantAdmin = errors.New("tenant must keep at least one administrator")
	// ErrTenantInactive means the tenant is disabled, so it cannot be entered.
	ErrTenantInactive = errors.New("tenant is disabled")
)

// ConstraintName returns the unique-index name behind a duplicate-key error.
// An error that is not a Postgres unique violation returns "".
func ConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

func wrapDuplicate(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	message := strings.ToLower(err.Error())
	if (errors.As(err, &pgErr) && pgErr.Code == "23505") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key") {
		return errors.Join(ErrDuplicateEntry, err)
	}
	return err
}
