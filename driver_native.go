//go:build native

package transwarp

import (
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/nativeadapter"
)

func init() {
	Register(DriverNative, func() Transwarp {
		return nativeadapter.New()
	})
}
