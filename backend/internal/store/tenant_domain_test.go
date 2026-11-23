package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "Example.COM:443", want: "example.com"},
		{in: "example.com.", want: "example.com"},
		{in: "  LocalHost  ", want: "localhost"},
		{in: "[::1]:5280", want: "::1"},
		{in: "ACME.LocalHost.:5280", want: "acme.localhost"},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		if got := NormalizeHost(tc.in); got != tc.want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugFromHost(t *testing.T) {
	slug, ok := SlugFromHost("Acme.Localhost:443", "LocalHost.")
	if !ok || slug != "acme" {
		t.Fatalf("slug host = %q ok=%v, want acme", slug, ok)
	}
	if _, ok := SlugFromHost("localhost", "localhost"); ok {
		t.Fatal("apex host must not yield a slug")
	}
	if _, ok := SlugFromHost("a.b.localhost", "localhost"); ok {
		t.Fatal("nested label must not yield a slug")
	}
	if _, ok := SlugFromHost("www.localhost", "localhost"); ok {
		t.Fatal("reserved label must not yield a slug")
	}
	if _, ok := SlugFromHost("acme.saas.local", ""); ok {
		t.Fatal("empty base must not yield a slug")
	}
}

func TestSlugFromCode(t *testing.T) {
	if got := SlugFromCode("ACME"); got != "acme" {
		t.Fatalf("ACME → %q, want acme", got)
	}
	if got := SlugFromCode("My_Tenant"); got != "mytenant" {
		t.Fatalf("My_Tenant → %q, want mytenant", got)
	}
	if got := SlugFromCode("A"); got != "" {
		t.Fatalf("short code → %q, want empty", got)
	}
	if got := SlugFromCode("API"); got != "" {
		t.Fatalf("reserved code → %q, want empty", got)
	}
}

func TestResolveByHostSlug(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).
		WithArgs("0").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).
		WithArgs("true").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`FROM sys_tenant_domain`).
		WithArgs("acme.localhost").
		WillReturnRows(sqlmock.NewRows([]string{"id", "slug", "name"}))
	mock.ExpectQuery(`WHERE slug = `).
		WithArgs("acme").
		WillReturnRows(sqlmock.NewRows([]string{"id", "slug", "name"}).
			AddRow(int64(3), "acme", "Acme"))
	mock.ExpectCommit()

	repo := NewTenantRepository(&DB{DB: db})
	repo.SetBaseDomain("localhost")
	id, slug, name, ok, err := repo.ResolveByHost(context.Background(), "ACME.LocalHost:5280")
	if err != nil {
		t.Fatalf("ResolveByHost: %v", err)
	}
	if !ok || id != 3 || slug != "acme" || name != "Acme" {
		t.Fatalf("resolved id=%d slug=%q name=%q ok=%v", id, slug, name, ok)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResolveByHostVerifiedDomain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).
		WithArgs("0").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).
		WithArgs("true").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`FROM sys_tenant_domain`).
		WithArgs("billing.example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "slug", "name"}).
			AddRow(int64(8), "other", "Billing"))
	mock.ExpectCommit()

	repo := NewTenantRepository(&DB{DB: db})
	repo.SetBaseDomain("localhost")
	id, slug, name, ok, err := repo.ResolveByHost(context.Background(), "Billing.Example.COM")
	if err != nil {
		t.Fatalf("ResolveByHost: %v", err)
	}
	if !ok || id != 8 || slug != "other" || name != "Billing" {
		t.Fatalf("resolved id=%d slug=%q name=%q ok=%v", id, slug, name, ok)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResolveByHostMiss(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).
		WithArgs("0").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).
		WithArgs("true").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`FROM sys_tenant_domain`).
		WithArgs("acme.localhost").
		WillReturnRows(sqlmock.NewRows([]string{"id", "slug", "name"}))
	mock.ExpectCommit()

	repo := NewTenantRepository(&DB{DB: db})
	_, _, _, ok, err := repo.ResolveByHost(context.Background(), "acme.localhost")
	if err != nil {
		t.Fatalf("ResolveByHost: %v", err)
	}
	if ok {
		t.Fatal("unconfigured base and unknown domain must not resolve")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
