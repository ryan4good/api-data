package access

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLUserRepositoryFindsLoginAndIdentityRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewMySQLUserRepository(db)
	rows := []string{"id", "email", "display_name", "password_hash", "platform_role", "status"}
	mock.ExpectQuery(regexp.QuoteMeta(findUserByEmailSQL)).WithArgs("admin@example.test").
		WillReturnRows(sqlmock.NewRows(rows).AddRow(testUserID, testEmail, "Admin", "$2a$04$hash", "admin", "active"))
	user, err := repository.FindByEmail(context.Background(), "admin@example.test")
	if err != nil || user.ID != testUserID || user.PasswordHash == "" {
		t.Fatalf("user=%#v err=%v", user, err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(findUserByIDSQL)).WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows(rows).AddRow(testUserID, testEmail, "Admin", nil, "admin", "active"))
	user, err = repository.FindByID(context.Background(), testUserID)
	if err != nil || user.Email != testEmail || user.PasswordHash != "" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLUserRepositoryMapsMissingUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(findUserByEmailSQL)).WithArgs("missing@example.test").WillReturnError(sql.ErrNoRows)
	_, err = NewMySQLUserRepository(db).FindByEmail(context.Background(), "missing@example.test")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("error=%v", err)
	}
}
