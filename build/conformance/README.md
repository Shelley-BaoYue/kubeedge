# Conformance CI

The dedicated `Conformance CI` workflow uses a shared suite workflow for three
test selections:

| Suite | Node shard | Coverage |
| --- | --- | --- |
| conformance | all | Existing conformance selection |
| nodeconformance | fast | Existing node selection except the two back-off tests |
| nodeconformance | slow | Back-off cap and back-off reset on image update |

Conformance and fast node coverage run on every pull request, including
documentation-only changes, so the required check is never left pending by
workflow path filters. The two back-off tests run for:

- PRs touching high-risk paths or carrying the `ci/full-conformance` label.
- Every push to `master`, including documentation-only merges.
- Daily runs at 02:23 UTC (10:23 UTC+8) and manual workflow dispatches.

The weekly compatibility workflow in `schedule.yml` is unchanged. Scheduled runs
use the repository's default branch and become active after the workflow is
merged there. Daily and post-merge runs provide deferred coverage, not a guarantee
that an ordinary PR passes the slow tests before merging.

High-risk paths conservatively include `edge/`, `common/`, `pkg/`,
`cloud/pkg/edgecontroller/`, `cloud/pkg/cloudhub/`, `vendor/`, `staging/`,
`build/`, `hack/`, `tests/`, `.github/`, Go dependency/workspace manifests and
Makefiles. Changes are computed with a merge-base diff of the event's base/head
SHAs, not a limited API file list. Deleted and renamed-out paths are included.
Detection errors fail the policy job rather than silently skipping coverage.
The source of truth is `.github/scripts/conformance-policy.cjs`.

Adding or removing a PR label triggers reevaluation. Unrelated labels do not force
slow coverage; removing the full-conformance label still runs slow tests if paths
require them. New PR events cancel superseded conformance runs for that PR.
Post-merge, scheduled and manual runs are not cancelled by subsequent runs.

**Branch protection:** replace the old conformance matrix checks with the fixed
`Conformance required` check. It requires successful policy selection and both
regular suites, plus a successful slow suite when requested. A skipped slow suite
is accepted only when policy explicitly returned false. Failed or cancelled jobs
never satisfy it. Neither suite uses `continue-on-error`, and matrix fail-fast is
disabled. The workflow does not modify branch protection or create the label.

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

Disk cleanup runs before images are pulled or built. It removes unused toolchains
and images but avoids the broad apt package removal step. Keep an eye on available
disk space when increasing dependencies or test images.
The reusable suite pulls build-tools directly rather than depending on an artifact
from Main CI, allowing the same preparation to run independently on every trigger.

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
node --test .github/scripts/conformance-policy.test.cjs
```

The tests check that the fast/slow filters are disjoint, retain representative
members of the original selection, and isolate only the two back-off tests.
They also check runner arguments and execute as part of the node test image build.
Policy tests run before selecting coverage and check event/label/path decisions,
diff arguments, error propagation, and required-check success/failure states.
