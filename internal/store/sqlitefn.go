package store

import (
	"crypto/md5"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("md5", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		sum := md5.Sum([]byte(fmt.Sprint(args[0])))
		return hex.EncodeToString(sum[:]), nil
	})
	sqlite.MustRegisterDeterministicScalarFunction("encode_base64", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		switch v := args[0].(type) {
		case nil:
			return nil, nil
		case []byte:
			return base64.StdEncoding.EncodeToString(v), nil
		case string:
			return base64.StdEncoding.EncodeToString([]byte(v)), nil
		default:
			return base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(v))), nil
		}
	})
}
