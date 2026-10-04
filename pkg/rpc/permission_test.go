package rpc

import (
	"testing"
)

// CheckPrincipal 是 CheckPermission 删除后的直接替代测试面。
func checkPerm(t *testing.T, role, method string, want bool) {
	t.Helper()
	if got := CheckPrincipal(PrincipalFromRole(role), method); got != want {
		t.Errorf("CheckPrincipal(%q, %q) = %v, want %v", role, method, got, want)
	}
}

func TestCheckPrincipalRoles(t *testing.T) {
	cases := []struct {
		role   string
		method string
		want   bool
	}{
		{RoleGuest, "common:getNodes", true},
		{RoleGuest, "getNodes", true},
		{RoleGuest, "admin:addClient", false},
		{RoleGuest, "client:report", false},
		{RoleClient, "client:report", true},
		{RoleClient, "admin:addClient", false},
		{RoleAdmin, "admin:addClient", true},
		{RoleAdmin, "client:report", false},
		{RoleAdmin, "common:getNodes", true},
		// 未知命名空间默认要求 admin
		{RoleGuest, "plugin:foo", false},
		{RoleAdmin, "plugin:foo", true},
	}
	for _, c := range cases {
		checkPerm(t, c.role, c.method, c.want)
	}
}

func TestAllowWildcardSpecificity(t *testing.T) {
	// 命名空间默认 admin，但更具体的方法级规则可放宽。
	Allow("acltest:*", RoleAdmin)
	Allow("acltest:public*", RoleGuest) // 前缀通配，比 "acltest:*" 更具体

	checkPerm(t, RoleGuest, "acltest:publicInfo", true)
	checkPerm(t, RoleGuest, "acltest:secret", false)

	// 精确规则优先于任何通配。
	Allow("acltest:secret", RoleClient)
	checkPerm(t, RoleClient, "acltest:secret", true)
	checkPerm(t, RoleGuest, "acltest:secret", false)
}

func TestAllowOverride(t *testing.T) {
	Allow("override:x", RoleAdmin)
	checkPerm(t, RoleGuest, "override:x", false)
	Allow("override:x", RoleGuest) // 覆盖同一 pattern
	checkPerm(t, RoleGuest, "override:x", true)
}

func TestInternalMethodsPermission(t *testing.T) {
	// 内部 rpc.* 方法对 guest 开放。
	checkPerm(t, RoleGuest, "rpc.ping", true)
	// 裸方法名归入 common，对 guest 开放。
	checkPerm(t, RoleGuest, "getNodes", true)
}
