package cli

import "errors"

// errNotImplemented is returned by Phase 0 subcommand stubs. Phase 1/2 will
// remove its usage as each subcommand becomes functional.
var errNotImplemented = errors.New("not yet implemented")
