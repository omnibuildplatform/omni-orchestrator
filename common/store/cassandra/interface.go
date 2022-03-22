package cadence

import "context"

type (
	Query interface {
		Exec() error
		Scan(...interface{}) error
		ScanCAS(...interface{}) (bool, error)
		MapScan(map[string]interface{}) error
		MapScanCAS(map[string]interface{}) (bool, error)
		Iter() Iter
		PageSize(int) Query
		PageState([]byte) Query
		WithContext(context.Context) Query
		WithTimestamp(int64) Query
		Bind(...interface{}) Query
	}

	Batch interface {
		Query(string, ...interface{})
		WithContext(context.Context) Batch
		WithTimestamp(int64) Batch
	}

	// Iter is the interface for executing and iterating over all resulting rows.
	Iter interface {
		Scan(...interface{}) bool
		MapScan(map[string]interface{}) bool
		PageState() []byte
		Close() error
	}
)
