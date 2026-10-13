package handlers

import (
	"encoding/json"
	"testing"

	"leiheV2/db"
)

func TestNormalizeIgnoredEmailAddresses(t *testing.T) {
	got, err := normalizeIgnoredEmailAddresses(" User@example.org,\nuser@example.org ; other@example.net ")
	if err != nil {
		t.Fatalf("normalize ignored email addresses: %v", err)
	}
	if want := "user@example.org\nother@example.net"; got != want {
		t.Errorf("normalized addresses = %q, want %q", got, want)
	}

	if _, err := normalizeIgnoredEmailAddresses("not-an-email"); err == nil {
		t.Error("expected invalid address to be rejected")
	}
}

func TestEmailSettingsControlDeliveryCategoriesAndIgnoredRecipients(t *testing.T) {
	settings := db.AppSettings{
		EmailBookingReceived:  false,
		EmailBookingReview:    true,
		EmailBookingUpdates:   false,
		IgnoredEmailAddresses: "noreply@example.org\nBLOCKED@example.net",
	}
	for _, test := range []struct {
		category emailCategory
		want     bool
	}{
		{emailBookingReceived, false},
		{emailBookingReview, true},
		{emailBookingUpdates, false},
		{"", true},
	} {
		if got := emailCategoryEnabled(settings, test.category); got != test.want {
			t.Errorf("emailCategoryEnabled(%q) = %t, want %t", test.category, got, test.want)
		}
	}
	for _, recipient := range []string{"NOREPLY@example.org", "blocked@example.net"} {
		if !ignoredEmailAddress(settings.IgnoredEmailAddresses, recipient) {
			t.Errorf("expected %q to be ignored", recipient)
		}
	}
	if ignoredEmailAddress(settings.IgnoredEmailAddresses, "user@example.org") {
		t.Error("unexpectedly ignored an address not in the list")
	}
}

func TestRoleFromOIDCGroups(t *testing.T) {
	auth = authConfig{AdminRole: "FIUS-Admin", MaintainerRole: "/maintainer", GroupClaims: []string{"groups"}}
	defer func() { auth = authConfig{} }()
	for _, test := range []struct {
		groups []string
		want   string
	}{
		{nil, "user"},
		{[]string{"other"}, "user"},
		{[]string{"maintainer"}, "maintainer"},
		{[]string{"/Maintainer", "fius-admin"}, "admin"},
		{[]string{"fius-admin", "maintainer"}, "admin"},
	} {
		if got := roleFromOIDC(test.groups); got != test.want {
			t.Errorf("roleFromOIDC(%v) = %q, want %q", test.groups, got, test.want)
		}
	}
	groups := oidcGroups(map[string]json.RawMessage{"groups": json.RawMessage(`["a","b"]`)})
	if len(groups) != 2 {
		t.Errorf("groups = %v", groups)
	}
	if groups := oidcGroups(map[string]json.RawMessage{"groups": json.RawMessage(`"a b"`)}); len(groups) != 2 {
		t.Errorf("string groups = %v", groups)
	}
}
