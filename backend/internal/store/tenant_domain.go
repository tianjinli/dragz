package store

import (
	"context"
	"database/sql"
	"net"
	"regexp"
	"strings"
	"time"
)

// slugPattern is the public host label: 2–63 chars, starting with a letter.
var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

// domainHostPattern is a lowercase hostname with no port and no trailing dot.
var domainHostPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// TenantDomain is one custom host bound to a tenant. verifiedAt is nil until
// an operator marks the row verified; only verified rows resolve a request.
type TenantDomain struct {
	ID         uint64     `json:"id"`
	TenantID   uint64     `json:"tenantId"`
	Host       string     `json:"host"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
	IsPrimary  int16      `json:"isPrimary"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// SetBaseDomain records the parent of slug subdomains. host "acme" plus base
// "localhost" resolves acme.localhost. An empty base leaves slug lookup off.
func (r *TenantRepository) SetBaseDomain(domain string) {
	if r == nil {
		return
	}
	r.baseDomain = NormalizeHost(domain)
}

// NormalizeHost strips a port and a trailing dot and lowercases the host.
// "Example.COM.:443" and "[::1]:5280" become "example.com" and "::1".
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimRight(host, ".")
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.TrimSpace(host)
	host = strings.TrimRight(host, ".")
	return strings.ToLower(host)
}

// SlugFromHost returns the single label in front of base. The apex host, a
// nested label (a.b.base), and reserved labels do not resolve.
func SlugFromHost(host, base string) (string, bool) {
	host = NormalizeHost(host)
	base = NormalizeHost(base)
	if host == "" || base == "" || host == base {
		return "", false
	}
	suffix := "." + base
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}
	label := strings.TrimSuffix(host, suffix)
	if label == "" || strings.Contains(label, ".") {
		return "", false
	}
	if SlugRejection(label) != "" {
		return "", false
	}
	return label, true
}

// SlugFromCode builds a slug from a tenant code. An empty result means the
// code cannot be a public label (too short, or a reserved word) and the
// caller should store no slug.
func SlugFromCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(code)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 63 {
		slug = strings.Trim(slug[:63], "-")
	}
	if SlugRejection(slug) != "" {
		return ""
	}
	return slug
}

// SlugRejection reports why slug cannot be published.
// "invalid" and "reserved" are the only values; "" means the slug is usable.
func SlugRejection(slug string) string {
	if !slugPattern.MatchString(slug) {
		return "invalid"
	}
	switch slug {
	case "www", "api", "admin", "app", "static":
		return "reserved"
	default:
		return ""
	}
}

// ValidDomainHost reports whether host is a normalized hostname we can store.
func ValidDomainHost(host string) bool {
	host = NormalizeHost(host)
	if host == "" || len(host) > 255 || strings.Contains(host, "..") {
		return false
	}
	if net.ParseIP(host) != nil {
		return false
	}
	return domainHostPattern.MatchString(host)
}

// ResolveByHost maps a request host to a live tenant.
//
// A verified custom domain wins. Otherwise, when a base domain is configured,
// {slug}.{base} loads that slug. The apex host itself does not: it is the
// shared platform name unless a verified domain row says otherwise.
// The read bypasses row-level security because it runs before a tenant exists.
func (r *TenantRepository) ResolveByHost(ctx context.Context, host string) (uint64, string, string, bool, error) {
	host = NormalizeHost(host)
	if host == "" || r == nil || r.db == nil || r.db.DB == nil {
		return 0, "", "", false, nil
	}
	var (
		id    uint64
		slug  string
		name  string
		found bool
	)
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		row := tx.QueryRowContext(ctx, `
SELECT t.id, t.slug, t.name
FROM sys_tenant_domain d
INNER JOIN sys_tenant t ON t.id = d.tenant_id
WHERE d.host = $1 AND d.verified_at IS NOT NULL AND t.deleted_at IS NULL`, host)
		matched, scanErr := scanResolvedTenant(row)
		if scanErr == nil {
			id, slug, name, found = matched.id, matched.slug, matched.name, true
			return nil
		}
		if scanErr != sql.ErrNoRows {
			return scanErr
		}
		label, ok := SlugFromHost(host, r.baseDomain)
		if !ok {
			return nil
		}
		row = tx.QueryRowContext(ctx, `
SELECT id, slug, name
FROM sys_tenant
WHERE slug = $1 AND deleted_at IS NULL`, label)
		matched, scanErr = scanResolvedTenant(row)
		if scanErr == sql.ErrNoRows {
			return nil
		}
		if scanErr != nil {
			return scanErr
		}
		id, slug, name, found = matched.id, matched.slug, matched.name, true
		return nil
	})
	if err != nil || !found {
		return 0, "", "", false, err
	}
	return id, slug, name, true, nil
}

type resolvedTenant struct {
	id   uint64
	slug string
	name string
}

func scanResolvedTenant(scanner interface{ Scan(...any) error }) (resolvedTenant, error) {
	var (
		id   int64
		slug string
		name string
	)
	if err := scanner.Scan(&id, &slug, &name); err != nil {
		return resolvedTenant{}, err
	}
	if id <= 0 {
		return resolvedTenant{}, sql.ErrNoRows
	}
	return resolvedTenant{id: uint64(id), slug: slug, name: name}, nil
}

// SetSlug publishes a tenant's subdomain label. A missing tenant is ErrNotFound.
// A live slug collision is ErrDuplicateEntry.
func (r *TenantRepository) SetSlug(ctx context.Context, tenantID uint64, slug string) error {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant SET slug = $2, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`, tenantID, slug)
		if err != nil {
			return wrapDuplicate(err)
		}
		affected, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// ListDomains returns every custom host for a live tenant, oldest id first.
// A missing tenant is ErrNotFound.
func (r *TenantRepository) ListDomains(ctx context.Context, tenantID uint64) ([]TenantDomain, error) {
	var items []TenantDomain
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		exists, err := tenantExists(ctx, tx, tenantID)
		if err != nil || !exists {
			if err == nil {
				err = ErrNotFound
			}
			return err
		}
		rows, err := tx.QueryContext(ctx, `
SELECT id, tenant_id, host, verified_at, is_primary, created_at
FROM sys_tenant_domain
WHERE tenant_id = $1
ORDER BY id`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		items = make([]TenantDomain, 0)
		for rows.Next() {
			item, err := scanTenantDomain(rows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []TenantDomain{}
	}
	return items, nil
}

// AddDomain stores an unverified host. A missing tenant is ErrNotFound.
// A host already bound to any tenant is ErrDuplicateEntry.
func (r *TenantRepository) AddDomain(ctx context.Context, tenantID uint64, host string) (TenantDomain, error) {
	host = NormalizeHost(host)
	var item TenantDomain
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		exists, err := tenantExists(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		row := tx.QueryRowContext(ctx, `
INSERT INTO sys_tenant_domain (tenant_id, host)
VALUES ($1, $2)
RETURNING id, tenant_id, host, verified_at, is_primary, created_at`, tenantID, host)
		scanned, scanErr := scanTenantDomain(row)
		if scanErr != nil {
			return wrapDuplicate(scanErr)
		}
		item = scanned
		return nil
	})
	return item, err
}

// MarkDomainVerified stamps verified_at. A missing row is ErrNotFound.
func (r *TenantRepository) MarkDomainVerified(ctx context.Context, tenantID, domainID uint64) error {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_domain
SET verified_at = NOW(), updated_at = NOW()
WHERE id = $1 AND tenant_id = $2`, domainID, tenantID)
		if err != nil {
			return err
		}
		affected, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// RemoveDomain deletes one host binding. A missing row is ErrNotFound.
func (r *TenantRepository) RemoveDomain(ctx context.Context, tenantID, domainID uint64) error {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
DELETE FROM sys_tenant_domain WHERE id = $1 AND tenant_id = $2`, domainID, tenantID)
		if err != nil {
			return err
		}
		affected, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

func tenantExists(ctx context.Context, tx *Tx, tenantID uint64) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM sys_tenant WHERE id = $1 AND deleted_at IS NULL
)`, tenantID).Scan(&exists)
	return exists, err
}

func scanTenantDomain(scanner interface{ Scan(...any) error }) (TenantDomain, error) {
	var (
		item     TenantDomain
		verified sql.NullTime
	)
	err := scanner.Scan(&item.ID, &item.TenantID, &item.Host, &verified, &item.IsPrimary, &item.CreatedAt)
	if err != nil {
		return TenantDomain{}, err
	}
	if verified.Valid {
		stamp := verified.Time
		item.VerifiedAt = &stamp
	}
	return item, nil
}
