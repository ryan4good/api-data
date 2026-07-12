package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
	"time"
)

func mysqlMock(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, m, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewMySQLRepository(db), m
}
func TestMySQLPromotionTransactionScopesAndPersistsOrderedSteps(t *testing.T) {
	repo, m := mysqlMock(t)
	m.ExpectBegin()
	m.ExpectQuery(regexp.QuoteMeta(lockCandidateSQL)).WithArgs(systemA, discoveryID, candidateID).WillReturnRows(sqlmock.NewRows(candidatePromotionColumns).AddRow(candidateID, systemA, discoveryID, "create-order", "创建订单", "P0", `{"priority":"P0","requiresReview":true,"sourceRefs":["prd"],"steps":[{"key":"pre","name":"查询","operationId":"55555555-5555-4555-8555-555555555555","method":"GET","path":"/orders"},{"key":"action","name":"创建","operationId":"66666666-6666-4666-8666-666666666666","method":"POST","path":"/orders"}]}`, "accepted", nil))
	m.ExpectExec(regexp.QuoteMeta(insertScenarioSQL)).WithArgs(sqlmock.AnyArg(), systemA, "create-order", "创建订单", "P0", "draft", userID, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(regexp.QuoteMeta(insertVersionSQL)).WithArgs(sqlmock.AnyArg(), systemA, sqlmock.AnyArg(), 1, "generated", sqlmock.AnyArg(), userID, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(regexp.QuoteMeta(insertStepSQL)).WithArgs(sqlmock.AnyArg(), systemA, sqlmock.AnyArg(), "pre", "查询", 1, "http", "55555555-5555-4555-8555-555555555555", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(regexp.QuoteMeta(insertStepSQL)).WithArgs(sqlmock.AnyArg(), systemA, sqlmock.AnyArg(), "action", "创建", 2, "http", "66666666-6666-4666-8666-666666666666", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(regexp.QuoteMeta(updateCurrentVersionSQL)).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), systemA, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(regexp.QuoteMeta(markCandidatePromotedSQL)).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), systemA, discoveryID, candidateID, "accepted").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	result, err := repo.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || len(result.Steps) != 2 {
		t.Fatalf("result=%#v", result)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestMySQLScenarioQueriesUseSystemScope(t *testing.T) {
	repo, m := mysqlMock(t)
	m.ExpectQuery(regexp.QuoteMeta(listScenariosSQL)).WithArgs(systemA).WillReturnRows(sqlmock.NewRows(scenarioColumns))
	if _, err := repo.List(context.Background(), systemA); err != nil {
		t.Fatal(err)
	}
	m.ExpectQuery(regexp.QuoteMeta(getScenarioSQL)).WithArgs(systemB, "scenario-x").WillReturnRows(sqlmock.NewRows(scenarioColumns))
	if _, found, err := repo.Get(context.Background(), systemB, "scenario-x"); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLPromotionRollsBackUnacceptedCandidate(t *testing.T) {
	repo, m := mysqlMock(t)
	m.ExpectBegin()
	m.ExpectQuery(regexp.QuoteMeta(lockCandidateSQL)).WithArgs(systemA, discoveryID, candidateID).
		WillReturnRows(sqlmock.NewRows(candidatePromotionColumns).AddRow(candidateID, systemA, discoveryID, "create-order", "创建订单", nil, `{"priority":"P0","requiresReview":true,"sourceRefs":[],"steps":[]}`, "pending", nil))
	m.ExpectRollback()
	_, err := repo.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if !errors.Is(err, ErrCandidateNotAccepted) {
		t.Fatalf("err=%v", err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLUpdateLocksScenarioAndCreatesManualVersionTransactionally(t *testing.T) {
	repo, m := mysqlMock(t)
	scenarioID := "77777777-7777-4777-8777-777777777777"
	oldVersionID := "88888888-8888-4888-8888-888888888888"
	now := time.Date(2026, 7, 12, 1, 2, 3, 0, time.UTC)
	m.ExpectBegin()
	m.ExpectQuery(regexp.QuoteMeta(lockScenarioSQL)).WithArgs(systemA, scenarioID).WillReturnRows(
		sqlmock.NewRows(scenarioColumns).AddRow(scenarioID, systemA, "create-order", "旧名称", nil, "draft", oldVersionID, userID, now, now),
	)
	m.ExpectQuery(regexp.QuoteMeta(nextVersionNoSQL)).WithArgs(systemA, scenarioID).WillReturnRows(sqlmock.NewRows([]string{"next_version_no"}).AddRow(2))
	m.ExpectExec(regexp.QuoteMeta(insertVersionSQL)).WithArgs(sqlmock.AnyArg(), systemA, scenarioID, 2, "manual", sqlmock.AnyArg(), userID, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(regexp.QuoteMeta(insertStepSQL)).WithArgs(sqlmock.AnyArg(), systemA, sqlmock.AnyArg(), "login", "登录", 1, "http", "", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(regexp.QuoteMeta(insertStepSQL)).WithArgs(sqlmock.AnyArg(), systemA, sqlmock.AnyArg(), "wait", "等待", 2, "delay", "", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectExec(regexp.QuoteMeta(updateScenarioRevisionSQL)).WithArgs("人工订单场景", "人工修订", "active", sqlmock.AnyArg(), sqlmock.AnyArg(), systemA, scenarioID, oldVersionID).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()

	got, err := repo.Update(context.Background(), systemA, scenarioID, userID, manualRevision())
	if err != nil {
		t.Fatal(err)
	}
	if got.Version.VersionNo != 2 || got.Version.SourceType != "manual" || got.Scenario.CurrentVersionID != got.Version.ID {
		t.Fatalf("detail=%#v", got)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLGetPreservesRequestConfigAsRawJSONObject(t *testing.T) {
	repo, m := mysqlMock(t)
	scenarioID := "77777777-7777-4777-8777-777777777777"
	versionID := "88888888-8888-4888-8888-888888888888"
	stepID := "99999999-9999-4999-8999-999999999999"
	now := time.Date(2026, 7, 12, 1, 2, 3, 0, time.UTC)
	m.ExpectQuery(regexp.QuoteMeta(getScenarioSQL)).WithArgs(systemA, scenarioID).WillReturnRows(
		sqlmock.NewRows(scenarioColumns).AddRow(scenarioID, systemA, "create-order", "订单", nil, "active", versionID, userID, now, now),
	)
	m.ExpectQuery(regexp.QuoteMeta(getVersionSQL)).WithArgs(systemA, versionID).WillReturnRows(
		sqlmock.NewRows([]string{"id", "system_id", "scenario_id", "version_no", "source_type", "bundle_document", "created_by", "created_at"}).
			AddRow(versionID, systemA, scenarioID, 2, "manual", `{}`, userID, now),
	)
	m.ExpectQuery(regexp.QuoteMeta(listStepsSQL)).WithArgs(systemA, versionID).WillReturnRows(
		sqlmock.NewRows([]string{"id", "system_id", "scenario_version_id", "step_key", "name", "position", "step_type", "api_operation_id", "depends_on", "request_config", "created_at"}).
			AddRow(stepID, systemA, versionID, "request", "请求", 1, "http", nil, `[]`, `{"method":"GET","path":"/orders"}`, now),
	)
	detail, found, err := repo.Get(context.Background(), systemA, scenarioID)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	payload, err := json.Marshal(detail.Steps[0])
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`"requestConfig":\{"method":"GET","path":"/orders"\}`).Match(payload) {
		t.Fatalf("requestConfig was not preserved as JSON: %s", payload)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
