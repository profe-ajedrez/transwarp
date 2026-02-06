package transwarp

import (
	"fmt"
)

// routerConstructor defines the signature for a factory function that creates
// a new instance of a Transwarp router.
//
// Concrete implementations (like the Fiber or Gin adapters) must match this
// signature to be stored in the registry.
type routerConstructor func() Transwarp

// registry is the central, internal repository of available driver constructors.
//
// It is populated at runtime during the package initialization phase (`init()`).
// Crucially, this map will ONLY contain drivers that were included in the
// binary via Go build tags (e.g., `-tags fiber`). If a driver file is excluded
// during compilation, its `init()` function never runs, and it is never added here.
var registry = make(map[Driver]routerConstructor)

// Register adds a driver constructor to the central registry.
//
// This function is intended to be called within the `init()` functions of the
// driver-specific files (e.g., `driver_fiber.go`). This allows for a decoupled,
// plugin-like architecture where the core logic doesn't strictly depend on
// specific frameworks until they are explicitly linked by the compiler.
//
// Parameters:
//   - d: The Driver enum key (e.g., DriverFiber).
//   - c: The constructor function that initializes that specific driver.
func Register(d Driver, c routerConstructor) {
	registry[d] = c
}

// New is the main factory method for instantiating the Transwarp engine.
//
// It looks up the requested driver in the internal registry and returns
// an initialized Transwarp interface.
//
// Panic:
// This function will panic if the requested Driver is not found in the registry.
// This usually happens when there is a mismatch between the code configuration
// and the compilation command.
//
// Example of a Panic Scenario:
//   - Code: transwarp.New(transwarp.Config{Driver: transwarp.DriverFiber})
//   - Command: go run -tags gin main.go
//
// In this case, the Fiber driver was never compiled, so it never registered itself,
// causing the lookup to fail. The command should has been:
//   - Code: transwarp.New(transwarp.Config{Driver: transwarp.DriverFiber})
//   - Command: go run .tags fiber main.go
func New(cfg Config) Transwarp {
	constructor, ok := registry[cfg.Driver]
	if !ok {
		// Provide a helpful error message guiding the user to the missing build tag.
		panic(fmt.Sprintf("Transwarp Error: Driver '%s' is not available. Did you forget to compile with '-tags %s'?", cfg.Driver, cfg.Driver))
	}
	return constructor()
}
