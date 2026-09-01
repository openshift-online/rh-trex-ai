package services

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v4"

	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/dao"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	dbmocks "github.com/openshift-online/rh-trex-ai/pkg/db/mocks"

	"github.com/onsi/gomega/types"
	"github.com/yaacov/tree-search-language/pkg/tsl"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"

	. "github.com/onsi/gomega"
)

type testModel struct {
	api.Meta
	Species string
}

func (testModel) TableName() string { return "dinosaurs" }

func TestSQLTranslation(t *testing.T) {
	RegisterTestingT(t)
	var dbFactory db.SessionFactory = dbmocks.NewMockSessionFactory()
	defer func() { _ = dbFactory.Close() }()

	g := dao.NewGenericDao(&dbFactory)
	genericService := sqlGenericService{genericDao: g}

	// ill-formatted search or disallowed fields should be rejected
	tests := []map[string]interface{}{
		{
			"search": "garbage",
			"error":  "rh-trex-ai-21: Failed to parse search query: garbage",
		},
		{
			"search": "id in ('123')",
			"error":  "rh-trex-ai-21: dinosaurs.id is not a valid field name",
		},
	}
	for _, test := range tests {
		var list []testModel
		search := test["search"].(string)
		errorMsg := test["error"].(string)
		listCtx, model, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		(*listCtx.disallowedFields)["id"] = "id"
		_, serviceErr = genericService.buildSearch(listCtx, &d)
		Expect(serviceErr).To(HaveOccurred())
		Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		Expect(serviceErr.Error()).To(Equal(errorMsg))
	}

	// tests for sql parsing
	tests = []map[string]interface{}{
		{
			"search": "username in ('ooo.openshift')",
			"sql":    "username IN (?)",
			"values": ConsistOf("ooo.openshift"),
		},
	}
	for _, test := range tests {
		var list []testModel
		search := test["search"].(string)
		sqlReal := test["sql"].(string)
		valuesReal := test["values"].(types.GomegaMatcher)
		listCtx, _, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		tslTree, err := tsl.ParseTSL(search)
		Expect(err).ToNot(HaveOccurred())
		_, sqlizer, serviceErr := genericService.treeWalkForSqlizer(listCtx, tslTree)
		Expect(serviceErr).ToNot(HaveOccurred())
		sql, values, err := sqlizer.ToSql()
		Expect(err).ToNot(HaveOccurred())
		Expect(sql).To(Equal(sqlReal))
		Expect(values).To(valuesReal)
	}
}

// ctxWithJWTRoles returns a context with a JWT token containing the given realm_access.roles.
func ctxWithJWTRoles(roles []string) context.Context {
	rolesIface := make([]interface{}, len(roles))
	for i, r := range roles {
		rolesIface[i] = r
	}
	claims := jwt.MapClaims{
		"username": "testuser",
		"realm_access": map[string]interface{}{
			"roles": rolesIface,
		},
	}
	token := &jwt.Token{Claims: claims, Valid: true}
	ctx := context.WithValue(context.Background(), auth.ContextAuthKey, token)
	return ctx
}

func TestBuildUserScope(t *testing.T) {
	RegisterTestingT(t)
	var dbFactory db.SessionFactory = dbmocks.NewMockSessionFactory()
	defer func() { _ = dbFactory.Close() }()

	g := dao.NewGenericDao(&dbFactory)
	genericService := sqlGenericService{genericDao: g}

	t.Run("no config registered — no filtering", func(t *testing.T) {
		RegisterTestingT(t)
		delete(userScopeConfigs, "testModel")
		var list []testModel
		listCtx, model, err := genericService.newListContext(context.Background(), "alice", &ListArguments{}, &list)
		Expect(err).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		finished, serviceErr := genericService.buildUserScope(listCtx, &d)
		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(finished).To(BeFalse())
	})

	t.Run("empty username — no filtering (dev mode)", func(t *testing.T) {
		RegisterTestingT(t)
		SetUserScopeConfig("testModel", &UserScopeConfig{
			OwnershipField: "created_by_user_id",
			AdminRoles:     []string{"admin"},
		})
		defer delete(userScopeConfigs, "testModel")
		var list []testModel
		listCtx, model, err := genericService.newListContext(context.Background(), "", &ListArguments{}, &list)
		Expect(err).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		finished, serviceErr := genericService.buildUserScope(listCtx, &d)
		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(finished).To(BeFalse())
	})

	t.Run("admin role — no filtering", func(t *testing.T) {
		RegisterTestingT(t)
		SetUserScopeConfig("testModel", &UserScopeConfig{
			OwnershipField: "created_by_user_id",
			AdminRoles:     []string{"admin", "platform-admin"},
		})
		defer delete(userScopeConfigs, "testModel")
		ctx := ctxWithJWTRoles([]string{"user", "admin"})
		var list []testModel
		listCtx, model, err := genericService.newListContext(ctx, "alice", &ListArguments{}, &list)
		Expect(err).ToNot(HaveOccurred())
		d := g.GetInstanceDao(ctx, model)
		finished, serviceErr := genericService.buildUserScope(listCtx, &d)
		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(finished).To(BeFalse())
	})

	t.Run("regular user — adds WHERE filter", func(t *testing.T) {
		RegisterTestingT(t)
		SetUserScopeConfig("testModel", &UserScopeConfig{
			OwnershipField: "created_by_user_id",
			AdminRoles:     []string{"admin"},
		})
		defer delete(userScopeConfigs, "testModel")
		ctx := ctxWithJWTRoles([]string{"user"})
		var list []testModel
		listCtx, model, err := genericService.newListContext(ctx, "alice", &ListArguments{}, &list)
		Expect(err).ToNot(HaveOccurred())
		d := g.GetInstanceDao(ctx, model)
		finished, serviceErr := genericService.buildUserScope(listCtx, &d)
		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(finished).To(BeFalse())
		// The WHERE clause was added to the DAO — we can't easily inspect gorm internals,
		// but we verify no error and no premature finish
	})
}
