# Conformance CI

The main CI workflow runs three independent matrix jobs:

| Suite | Node shard | Coverage |
| --- | --- | --- |
| conformance | all | Existing conformance selection |
| nodeconformance | fast | Existing node selection except the two back-off tests |
| nodeconformance | slow | Back-off cap and back-off reset on image update |

Both node shards run on every pull request. Neither shard is allowed to fail,
and matrix fail-fast is disabled so that one failure does not cancel the other
results. When adopting these jobs, update branch protection's required check
names to include both node shards. The workflow does not modify branch protection.

The back-off cap test takes about 27 minutes by design. Isolating it lets it start
without waiting behind randomly ordered tests; it does not shorten its observation
windows. Other `[Slow]` tests remain in the fast shard. Existing skip rules,
including `[Serial]`, still apply to every shard.

Local `make conformance_e2e` keeps the full node suite by default. To run a shard:

```sh
make conformance_e2e KIND_IMAGE=kindest/node:v1.32.2 CONFORMANCE_TYPE=nodeconformance NODE_CONFORMANCE_SHARD=fast
make conformance_e2e KIND_IMAGE=kindest/node:v1.32.2 CONFORMANCE_TYPE=nodeconformance NODE_CONFORMANCE_SHARD=slow
```

`NODE_CONFORMANCE_SHARD=all` preserves custom `E2E_FOCUS` when invoking the node
runner directly. Fast and slow shards reject custom focus expressions to avoid
silently changing the partition. When updating Kubernetes dependencies, check
that the two back-off test names still match `slowNodeFocus` in the node runner.
Sharded runs fail rather than report success if their test selection is empty.

## Preparation and caching

CI checks out the source before setup-go computes dependency hashes. Component
builds use a separate cache for `_output/local/_go/cache`, the path used inside
the build-tools container through the workspace mount. Cache keys include
dependency files, the build-tools configuration, and the commit; restore keys
allow unchanged compiled packages to be reused by subsequent commits.

Disk cleanup runs before images are loaded or built. It removes unused toolchains
and images but avoids the broad apt package removal step. Keep an eye on available
disk space when increasing dependencies or test images.

CI builds the test image from the clean checkout with Buildx and a per-suite
GitHub Actions layer cache, then passes its local tag as `CONFORMANCE_IMAGE`.
The script checks that this image exists and does not rebuild it. Without that
variable, local runs continue to build the image themselves. Node shards do not
build the mapper; the conformance suite still does because it contains mapper tests.

Each test Dockerfile has its own ignore file, excluding Git data, component build
outputs/caches, and generated mapper projects without changing other image builds.
The source `vendor` and `staging` trees remain available. Buildx caches image
layers, not a cross-run Go compiler cache inside those layers: source changes
that invalidate the compilation layer can still require a test binary rebuild.

## Focused regression tests

```sh
go test ./build/conformance/node-e2e-runner
```

The tests check that the fast/slow filters are disjoint, retain representative
members of the original selection, and isolate only the two back-off tests.
They also check runner arguments and execute as part of the node test image build.
