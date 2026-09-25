# DCK version

This directory contains the construction-kit version of teamg1-demo. The original Go sources are preserved at their original paths (revision `83e64521daea1a977cac481c9b652d2bcb86a248`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/teamg1demo` and this version with `go run ./dck/cmd/teamg1demo` from the repository root.

The choreography and assets remain in this repository. Reusable rendering and
effects come from the published `github.com/olivierh59500/democonstructionkit`
module pinned in `go.mod`. Music is opened with `sound.Open`; DCK selects the decoder from the asset and
provides the configured stereo PCM format. The demo keeps its playback level and loop settings.

The intro/main switch, 0.03-per-tick fade and strict 0.1 music cue use DCK's
`timeline.IntroHandoff`. The CRT pass and main effects retain their original
update and draw order.
