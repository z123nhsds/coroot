package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMCPPermissionsForLogClustering(t *testing.T) {
	viewer := NewPermission(ScopeProjectAll, ActionView, nil)
	assert.True(t, viewer.allows(Actions.Project("*").Node("*").View()))
	assert.False(t, viewer.allows(Actions.Project("*").Settings().Edit()))

	editor := NewPermission(ScopeProjectAll, ActionEdit, nil)
	assert.True(t, editor.allows(Actions.Project("*").Settings().Edit()))
	assert.False(t, editor.allows(Actions.Project("*").Node("*").View()))

	allViewer := NewPermission(ScopeAll, ActionView, nil)
	assert.True(t, allViewer.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, allViewer.allows(Actions.Settings().Edit()))
}

func TestMCPProjectScopedPermissions(t *testing.T) {
	viewerProjectScoped := NewPermission(ScopeProjectAll, ActionView, Object{"project_id": "prod-cluster"})
	assert.True(t, viewerProjectScoped.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, viewerProjectScoped.allows(Actions.Project("prod-cluster").Node("any-node").View()))
	assert.False(t, viewerProjectScoped.allows(Actions.Project("staging-cluster").Node("any-node").View()))
}

func TestMCPNodeScopePermissions(t *testing.T) {
	nodeViewer := NewPermission(ScopeNode, ActionView, Object{"node_name": "prod-*"})
	assert.True(t, nodeViewer.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, nodeViewer.allows(Actions.Project("cluster-1").Node("prod-node-1").View()))
	assert.True(t, nodeViewer.allows(Actions.Project("cluster-1").Node("prod-node-2").View()))
	assert.False(t, nodeViewer.allows(Actions.Project("cluster-1").Node("staging-node-1").View()))
}

func TestMCPApplicationScopePermissions(t *testing.T) {
	app := NewPermission(ScopeApplication, ActionView, Object{
		"application_id": "cluster-1:default:Deployment:api-gateway",
	})
	assert.True(t, app.allows(Actions.Project("*").Application("*", "*", "*", "*").View()))
	assert.False(t, app.allows(Actions.Project("*").Node("*").View()))
}

func TestMCPRolePermissions(t *testing.T) {
	role := &Role{
		Name: "LogViewer",
		Permissions: PermissionSet{
			NewPermission(ScopeProjectAll, ActionView, nil),
		},
	}
	assert.True(t, role.Permissions.Allows(Actions.Project("*").Node("*").View()))
	assert.False(t, role.Permissions.Allows(Actions.Project("*").Settings().Edit()))
}

func TestMCPEmptyPermissionDeniesAll(t *testing.T) {
	p := NewPermission(ScopeNode, ActionView, Object{"node_name": ""})
	assert.False(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.False(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("bar").View()))
}

func TestMCPNilPermission(t *testing.T) {
	p := NewPermission(ScopeAll, ActionView, nil)
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("any-project").Node("any-node").View()))
}

func TestMCPWildcardPermissionAllowsAll(t *testing.T) {
	p := NewPermission(ScopeNode, ActionView, Object{"node_name": "*"})
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("a").Node("b").View()))
	assert.True(t, p.allows(Actions.Project("x").Node("y").View()))
}

func TestMCPApplicationCategoryPermission(t *testing.T) {
	p := NewPermission(ScopeProjectApplicationCategories, ActionEdit, nil)
	assert.True(t, p.allows(Actions.Project("*").ApplicationCategories().Edit()))
	assert.False(t, p.allows(Actions.Project("*").ApplicationCategories().View()))
}

func TestMCPAlertsPermission(t *testing.T) {
	p := NewPermission(ScopeProjectAlerts, ActionEdit, nil)
	assert.True(t, p.allows(Actions.Project("*").Alerts().Edit()))
	assert.False(t, p.allows(Actions.Project("*").Alerts().View()))
}

func TestMCPRisksPermission(t *testing.T) {
	p := NewPermission(ScopeProjectRisks, ActionEdit, nil)
	assert.True(t, p.allows(Actions.Project("*").Risks().Edit()))
	assert.False(t, p.allows(Actions.Project("*").Risks().View()))
}

func TestMCPStaticRoleManager(t *testing.T) {
	mgr := NewStaticRoleManager()
	roles, err := mgr.GetRoles()
	assert.NoError(t, err)
	assert.Len(t, roles, 3)

	names := map[RoleName]bool{}
	for _, r := range roles {
		names[r.Name] = true
	}
	assert.True(t, names[RoleAdmin])
	assert.True(t, names[RoleEditor])
	assert.True(t, names[RoleViewer])
}

func TestMCPAdminRoleHasFullAccess(t *testing.T) {
	mgr := NewStaticRoleManager()
	roles, _ := mgr.GetRoles()
	var admin *Role
	for i := range roles {
		if roles[i].Name == RoleAdmin {
			admin = &roles[i]
			break
		}
	}
	assert.NotNil(t, admin)
	assert.True(t, admin.Permissions.Allows(Actions.Project("*").Node("*").View()))
	assert.True(t, admin.Permissions.Allows(Actions.Project("*").Settings().Edit()))
	assert.True(t, admin.Permissions.Allows(Actions.Users().Edit()))
}

func TestMCPViewerRoleHasViewOnly(t *testing.T) {
	mgr := NewStaticRoleManager()
	roles, _ := mgr.GetRoles()
	var viewer *Role
	for i := range roles {
		if roles[i].Name == RoleViewer {
			viewer = &roles[i]
			break
		}
	}
	assert.NotNil(t, viewer)
	assert.True(t, viewer.Permissions.Allows(Actions.Project("*").Node("*").View()))
	assert.False(t, viewer.Permissions.Allows(Actions.Project("*").Settings().Edit()))
}