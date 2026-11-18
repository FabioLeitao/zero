// Package buildlocal holds the offline smoke test of scripts/zero-databoar-build,
// the script that stamps and installs the fork's local build. There is no code
// here: the package exists so that `go test ./...` runs the script's label and
// version logic on every gate run, without a network, a model or an install.
package buildlocal
