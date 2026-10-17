# Mutation runner

This module pins go-mutesting and a current `golang.org/x/tools` package loader
without adding tool dependencies to the application module. The runner's original
2019 loader panics on current Go toolchains.

The local `osutil` replacement supplies only `CopyFile`, the sole operation
used by the pinned runner. The original dependency includes unrelated Unix-only
syscalls that prevent Windows compilation. Adapter tests run before building
the runner on either platform. Recheck its required API when updating the runner.

`../mutation-test.sh` fails on runner errors, surviving mutants, and selections
that generate no mutations. It runs in the caller's checkout, so use a disposable
clone or Codespace.

## Equivalent mutation

`../mutation-equivalents.txt` contains one reviewed checksum:

- `b342b564933b7740efc9cfc667c0db35` removes the first JSON decoding error
  return in `pkg/types.unmarshalContentBlock`. Invalid JSON is rejected again
  by the selected block decoder. A valid scalar or array fails both the map
  decoder and the fallback struct decoder. Both versions return a decoding
  error with no content block; the internal decoder's error wording can differ.
  No exported contract relies on that wording. The checksum is tied to this
  precise mutation; revisit it when the decoder changes.

This exclusion does not cover the block-specific error returns or the individual
tool field checks. Dedicated tests kill those mutations.
