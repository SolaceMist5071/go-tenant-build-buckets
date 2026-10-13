package main

import "testing"

func TestTenantBucketAndReleaseDecision(t *testing.T) {
	cases := []struct{ tenant, want string }{{"Acme Pay", "tenant-acme-pay"}, {"risk", "tenant-risk"}}
	for _, tc := range cases {
		if got := tenantBucket(tc.tenant); got != tc.want {
			t.Fatalf("tenantBucket(%q)=%q", tc.tenant, got)
		}
	}
}
