package regression

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"github.com/stretchr/testify/assert"
)

func TestUtilsFunctions_WatchersPrometheusHealthProbe(t *testing.T) {
	t.Run("GlobMatchSupportsAlertingRuleApplicationPatterns", func(t *testing.T) {
		assert.True(t, utils.GlobMatch("default:Deployment:checkout", "*"),
			"GlobMatch with wildcard must match any application ID pattern")

		assert.True(t, utils.GlobMatch("default:Deployment:checkout", "default:Deployment:*"),
			"GlobMatch must match namespace+kind wildcard pattern used in alerting rules")

		assert.True(t, utils.GlobMatch("default:Deployment:checkout-api", "default:Deployment:checkout*"),
			"GlobMatch must match prefix wildcard pattern for application name matching")

		assert.False(t, utils.GlobMatch("default:Deployment:payment", "default:Deployment:checkout*"),
			"GlobMatch must not match non-matching application names")
	})

	t.Run("GlobMatchSupportsRbacScopePatterns", func(t *testing.T) {
		assert.True(t, utils.GlobMatch("project.node", "*"),
			"GlobMatch with wildcard must match any RBAC scope")

		assert.True(t, utils.GlobMatch("project.node", "project.*"),
			"GlobMatch must match project-scoped wildcard for RBAC")

		assert.True(t, utils.GlobMatch("project.node", "project.node"),
			"GlobMatch must match exact RBAC scope")

		assert.False(t, utils.GlobMatch("project.node", "project.application"),
			"GlobMatch must not match different RBAC scopes")
	})

	t.Run("GlobMatchSupportsRbacActionPatterns", func(t *testing.T) {
		assert.True(t, utils.GlobMatch("view", "*"),
			"GlobMatch with wildcard must match any RBAC action verb")

		assert.True(t, utils.GlobMatch("view", "view"),
			"GlobMatch must match exact RBAC action verb")

		assert.False(t, utils.GlobMatch("view", "edit"),
			"GlobMatch must not match different RBAC action verbs")
	})

	t.Run("GlobMatchSupportsRbacObjectPatterns", func(t *testing.T) {
		assert.True(t, utils.GlobMatch("foo", "*"),
			"GlobMatch with wildcard must match any RBAC object value")

		assert.True(t, utils.GlobMatch("foo", "foo*"),
			"GlobMatch must match prefix wildcard for RBAC object values")

		assert.True(t, utils.GlobMatch("foobar", "foo*"),
			"GlobMatch must match prefix wildcard for RBAC object values")

		assert.False(t, utils.GlobMatch("bar", "foo*"),
			"GlobMatch must not match non-matching RBAC object values")
	})

	t.Run("GlobValidateRejectsInvalidPatterns", func(t *testing.T) {
		assert.True(t, utils.GlobValidate([]string{"*"}), "valid wildcard pattern must pass validation")
		assert.True(t, utils.GlobValidate([]string{"default:Deployment:*"}), "valid pattern must pass validation")
		assert.False(t, utils.GlobValidate([]string{"["}), "invalid pattern must fail validation")
	})

	t.Run("AlertingRuleMatchesUsesGlobMatch", func(t *testing.T) {
		app := model.NewApplication(model.NewApplicationId("c1", "default", model.ApplicationKindDeployment, "checkout"))
		app.Category = model.ApplicationCategoryApplication

		ruleAll := &model.AlertingRule{
			Selector: model.AppSelector{Type: model.AppSelectorTypeAll},
		}
		assert.True(t, ruleAll.Matches(app), "all-type selector must match any application")

		ruleCategory := &model.AlertingRule{
			Selector: model.AppSelector{
				Type:       model.AppSelectorTypeCategory,
				Categories: []string{string(model.ApplicationCategoryApplication)},
			},
		}
		assert.True(t, ruleCategory.Matches(app), "category selector must match application category")

		rulePattern := &model.AlertingRule{
			Selector: model.AppSelector{
				Type:                  model.AppSelectorTypeApplications,
				ApplicationIdPatterns: []string{"default:Deployment:*"},
			},
		}
		assert.True(t, rulePattern.Matches(app), "pattern selector must match via GlobMatch")
	})

	t.Run("RbacPermissionAllowsUsesGlobMatch", func(t *testing.T) {
		p := rbac.NewPermission(rbac.ScopeNode, rbac.ActionView, rbac.Object{"node_name": "foo*"})
		assert.True(t, p.Allows(rbac.Actions.Project("p1").Node("foobar").View()),
			"RBAC permission with GlobMatch pattern must allow matching node access")
		assert.False(t, p.Allows(rbac.Actions.Project("p1").Node("barbaz").View()),
			"RBAC permission with GlobMatch pattern must deny non-matching node access")
	})

	t.Run("PromFilterLabelsKeepAllAllowsHealthProbeMetrics", func(t *testing.T) {
		assert.True(t, prom.FilterLabelsKeepAll("up"), "FilterLabelsKeepAll must allow 'up' metric for health probes")
		assert.True(t, prom.FilterLabelsKeepAll("__name__"), "FilterLabelsKeepAll must allow __name__ label")
		assert.True(t, prom.FilterLabelsKeepAll("instance"), "FilterLabelsKeepAll must allow instance label")
	})

	t.Run("PromFilterLabelsDropAllBlocksAllLabels", func(t *testing.T) {
		assert.False(t, prom.FilterLabelsDropAll("up"), "FilterLabelsDropAll must block all labels")
		assert.False(t, prom.FilterLabelsDropAll("__name__"), "FilterLabelsDropAll must block all labels")
	})

	t.Run("NanoIdGeneratesUniqueIdsForAlerts", func(t *testing.T) {
		ids := map[string]bool{}
		for i := 0; i < 100; i++ {
			id := utils.NanoId(12)
			assert.Len(t, id, 12, "NanoId must generate IDs of specified length")
			assert.False(t, ids[id], "NanoId must generate unique IDs")
			ids[id] = true
		}
	})

	t.Run("TruncatePreservesAlertDetailIntegrity", func(t *testing.T) {
		short := "error in checkout service"
		assert.Equal(t, short, utils.Truncate(short, 100), "short strings must not be truncated")

		long := ""
		for i := 0; i < 200; i++ {
			long += "a"
		}
		truncated := utils.Truncate(long, 100)
		assert.Equal(t, 101, len([]rune(truncated)), "truncated string must be maxLen + ellipsis")
		assert.Equal(t, "…", string([]rune(truncated)[len([]rune(truncated))-1:]), "truncated string must end with ellipsis")
	})

	t.Run("StringSetOperationsForStatsCollection", func(t *testing.T) {
		ss := utils.NewStringSet()
		ss.Add("redis")
		ss.Add("postgres")
		ss.Add("redis")

		assert.Equal(t, 2, ss.Len(), "StringSet must deduplicate items")
		assert.True(t, ss.Has("redis"), "StringSet must report membership")
		assert.False(t, ss.Has("mysql"), "StringSet must report non-membership")

		items := ss.Items()
		assert.Equal(t, []string{"postgres", "redis"}, items, "StringSet items must be sorted")
	})

	t.Run("FormatFunctionsConsistentForPrometheusHealthProbeOutput", func(t *testing.T) {
		assert.Equal(t, "0", utils.FormatFloat(0), "FormatFloat zero")
		assert.Equal(t, "99", utils.FormatFloat(99), "FormatFloat >= 1")
		assert.Equal(t, "0.5", utils.FormatFloat(0.5), "FormatFloat >= 0.1")
		assert.Equal(t, "0.05", utils.FormatFloat(0.05), "FormatFloat >= 0.01")
		assert.Equal(t, "99%", utils.FormatPercentage(99), "FormatPercentage whole number")
		assert.Equal(t, "50%", utils.FormatPercentage(50), "FormatPercentage whole number")

		v, u := utils.FormatBytes(float32(100 * 1000 * 1000))
		assert.Equal(t, "100", v, "FormatBytes value")
		assert.Equal(t, "MB", u, "FormatBytes unit")
	})
}
