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
The shared intro feed inserts glyphs at the authored x=640 stage edge and
shifts its 768-pixel surface by six pixels per update. Opt-in captures of the
surface before CRT are byte-identical to the preserved Go original at frames
0, 1, 60, 240 and 600. The flat DCK CRT intentionally changes the final intro
image to keep the font's outer rows visible. Complete main-scene frames at
1,200, 2,400 and 4,800 are pixel-identical to the original. The updated DCK
APK was installed on Pixel 10a and the intro/main transition inspected; 744
presented intervals had p95 16.736 ms, maximum 16.930 ms and none over 20 ms.
Reproduce the raw intro comparison with the `teamg1_intro_sourcecheck` build
tag in the root and `dck` packages, setting `TEAMG1_INTRO_SOURCE_CAPTURES` to
different output directories before each run.
