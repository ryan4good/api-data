package scenario

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
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
