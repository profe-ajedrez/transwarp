//go:build chi || (!fiber && !gin && !echo)

package transwarp

import (
	"github.com/go-chi/chi/v5"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/chiadapter"
)

// init serves as the entry point for the Chi driver integration.
//
// It utilizes the "Self-Registration Pattern" to inject the Chi constructor
// into the central registry during the package initialization phase.
//
// Behavior:
// This function is executed automatically when the application starts, BUT
// only if the build constraints at the top of this file are met.
//
// Default Fallback Mechanism:
// The build tag logic `(!fiber && !gin && !echo)` ensures that if the developer
// compiles the application without providing any specific `-tags` flag,
// Transwarp will default to using Chi.
//
// This allows developers to run `go run main.go` and have a working server
// immediately (using Chi), while reserving the specific flags for when they
// explicitly want to switch engines (e.g., `go run -tags fiber main.go`).
func init() {
	Register(DriverChi, func() Transwarp {
		// Instantiate the native Chi router (v5).
		// Chi is chosen as the default because it is 100% compatible with
		// net/http and has a very small footprint.
		c := chi.NewRouter()

		// Wrap the native router in the Transwarp adapter to satisfy the
		// Transwarp interface.
		return &chiadapter.ChiAdapter{Router: c}
	})
}
