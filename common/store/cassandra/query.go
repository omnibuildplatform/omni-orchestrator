package cassandra

import (
	"context"

	"github.com/gocql/gocql"
)

var _ Query = (*query)(nil)

type (
	query struct {
		*gocql.Query

		session *Session
	}
)

func newQuery(
	session *Session,
	gocqlQuery *gocql.Query,
) *query {
	return &query{
		Query:   gocqlQuery,
		session: session,
	}
}

func (q *query) Exec() error {
	err := q.Query.Exec()
	return q.handleError(err)
}

func (q *query) Scan(
	dest ...interface{},
) error {
	err := q.Query.Scan(dest...)
	return q.handleError(err)
}

func (q *query) ScanCAS(
	dest ...interface{},
) (bool, error) {
	applied, err := q.Query.ScanCAS(dest...)
	return applied, q.handleError(err)
}

func (q *query) MapScan(
	m map[string]interface{},
) error {
	err := q.Query.MapScan(m)
	return q.handleError(err)
}

func (q *query) MapScanCAS(
	dest map[string]interface{},
) (bool, error) {
	applied, err := q.Query.MapScanCAS(dest)
	return applied, q.handleError(err)
}

func (q *query) Iter() Iter {
	iter := q.Query.Iter()
	if iter == nil {
		return nil
	}
	return iter
}

func (q *query) PageSize(n int) Query {
	q.Query.PageSize(n)
	return q
}

func (q *query) PageState(state []byte) Query {
	q.Query.PageState(state)
	return q
}

func (q *query) WithTimestamp(timestamp int64) Query {
	q.Query.WithTimestamp(timestamp)
	return q
}

func (q *query) WithContext(ctx context.Context) Query {
	q2 := q.Query.WithContext(ctx)
	if q2 == nil {
		return nil
	}
	return newQuery(q.session, q2)
}

func (q *query) Bind(v ...interface{}) Query {
	q.Query.Bind(v...)
	return q
}

func (q *query) handleError(err error) error {
	return q.session.handleError(err)
}
