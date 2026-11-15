package sqlite

import (
	"database/sql/driver"
	"modernc.org/sqlite"
	"strings"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("unicode_lower", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		value, _ := args[0].(string)
		return strings.ToLower(value), nil
	})
}
