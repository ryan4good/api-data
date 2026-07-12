package environment

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

func mockRepo(t *testing.T) (*MySQLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, m, e := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewMySQLRepository(db), m
}
func TestMySQLQueriesAreSystemScopedAndStoreOnlyReference(t *testing.T) {
	r, m := mockRepo(t)
	m.ExpectQuery(regexp.QuoteMeta(getEnvironmentSQL)).WithArgs(sysA, envID).WillReturnRows(sqlmock.NewRows(environmentColumns))
	if _, found, err := r.GetEnvironment(context.Background(), sysA, envID); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	ref := SecretReference{ID: "ref-id", SystemID: sysA, EnvironmentID: envID, VariableKey: "token", SecretRef: "vault://test/token", CreatedBy: userID}
	m.ExpectExec(regexp.QuoteMeta(upsertSecretReferenceSQL)).WithArgs("ref-id", sysA, envID, "token", "vault://test/token", userID).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := r.UpsertSecretReference(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	m.ExpectQuery(regexp.QuoteMeta(listSecretReferencesSQL)).WithArgs(sysA, envID).WillReturnRows(sqlmock.NewRows(secretReferenceColumns))
	if _, err := r.ListSecretReferences(context.Background(), sysA, envID); err != nil {
		t.Fatal(err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
