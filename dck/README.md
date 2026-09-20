# DCK version

This directory contains the construction-kit version of teamg1-demo. The original Go sources are preserved at their original paths (revision `83e64521daea1a977cac481c9b652d2bcb86a248`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/teamg1demo` and this version with `go run ./dck/cmd/teamg1demo` from the repository root.

The choreography and assets stay local; reusable rendering and effects live in `../../lib/democonstructionkit`. Second Reality retains its original ST3 music synchronization.
