/*
Copyright 2026 The KubeEdge Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

'use strict';

const assert = require('node:assert/strict');
const { execFileSync, spawnSync } = require('node:child_process');
const { mkdtempSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { join } = require('node:path');
const { test } = require('node:test');
const { changedFiles, checkResults, isHighRisk, selectCoverage } = require('./conformance-policy.cjs');

const base = 'a'.repeat(40);
const head = 'b'.repeat(40);
function pr(labels = []) {
  return { pull_request: { base: { sha: base }, head: { sha: head }, labels } };
}

test('ordinary PRs defer only slow coverage', () => {
  for (const action of ['opened', 'synchronize', 'reopened', 'labeled', 'unlabeled']) {
    assert.equal(selectCoverage('pull_request', { ...pr(), action },
      () => ['README.md', 'docs/guide.md', 'keadm/cmd/keadm/app/cmd.go']).runSlow, false);
  }
});

test('high-risk paths require slow coverage', () => {
  for (const path of [
    'edge/pkg/edged/edged.go', 'edge/pkg/metamanager/process.go',
    'common/types/types.go', 'pkg/apis/config/types.go',
    'cloud/pkg/edgecontroller/controller.go', 'cloud/pkg/cloudhub/server.go',
    'vendor/k8s.io/kubernetes/pkg/kubelet/kubelet.go',
    'vendor/github.com/containerd/containerd/client.go',
    'staging/src/github.com/kubeedge/api/go.mod',
    'go.mod', 'go.sum', 'go.work', 'go.work.sum', 'new-module/go.sum',
    'build/conformance/nodeconformance.Dockerfile', 'build/tools/Dockerfile',
    'hack/local-up-kubeedge.sh', 'tests/scripts/conformance_e2e.sh',
    '.github/workflows/conformance.yml', '.github/scripts/conformance-policy.cjs',
    'Makefile', 'build.mk',
  ]) {
    assert.equal(isHighRisk(path), true, path);
    assert.equal(selectCoverage('pull_request', pr(), () => [path]).runSlow, true, path);
  }
  assert.equal(isHighRisk('docs/go.mod.md'), false);
});

test('explicit label forces coverage and removal reevaluates paths', () => {
  assert.equal(selectCoverage('pull_request', pr([{ name: 'ci/full-conformance' }]),
    () => { throw new Error('Label should force coverage without a diff'); }).runSlow, true);
  assert.equal(selectCoverage('pull_request', pr([{ name: 'bug' }]), () => ['README.md']).runSlow, false);
  assert.equal(selectCoverage('pull_request', pr(), () => ['edge/pkg/edged/edged.go']).runSlow, true);
});

test('post-merge, scheduled and manual runs always include slow tests', () => {
  for (const event of ['push', 'schedule', 'workflow_dispatch']) {
    assert.equal(selectCoverage(event, {}, () => { throw new Error('Unexpected diff'); }).runSlow, true);
  }
});

test('diff uses merge base, includes deleted paths, and preserves unusual file names', () => {
  const files = changedFiles(base, head, (command, args, options) => {
    assert.equal(command, 'git');
    assert.deepEqual(args, [
      'diff', '--no-ext-diff', '--no-renames', '--name-only', '-z', `${base}...${head}`, '--',
    ]);
    assert.equal(options.encoding, 'utf8');
    return 'edge/old.go\0docs/new name.go\0docs/line\nbreak.md\0';
  });
  assert.deepEqual(files, ['edge/old.go', 'docs/new name.go', 'docs/line\nbreak.md']);
  assert.equal(selectCoverage('pull_request', pr(), () => files).runSlow, true);
});

test('missing metadata and detection errors fail closed', () => {
  for (const sha of [undefined, '', '--help', 'not-a-commit']) {
    assert.throws(() => changedFiles(sha, head), /SHA/);
    assert.throws(() => changedFiles(base, sha), /SHA/);
  }
  assert.throws(() => selectCoverage('pull_request', {}), /metadata/);
  assert.throws(() => selectCoverage('pull_request_target', pr()), /Unsupported/);
  assert.throws(() => selectCoverage('pull_request', pr(), () => {
    throw new Error('git diff failed');
  }), /git diff failed/);
  assert.throws(() => changedFiles(base, head, () => {
    throw new Error('missing merge base');
  }), /missing merge base/);
});

test('required check accepts only the intended complete coverage', () => {
  checkResults({ policy: 'success', fast: 'success', slow: 'success', runSlow: 'true' });
  checkResults({ policy: 'success', fast: 'success', slow: 'skipped', runSlow: 'false' });
  for (const result of ['failure', 'cancelled', 'skipped', '']) {
    assert.throws(() => checkResults({
      policy: result, fast: 'success', slow: 'skipped', runSlow: 'false',
    }), /policy/);
    assert.throws(() => checkResults({
      policy: 'success', fast: result, slow: 'success', runSlow: 'true',
    }), /fast node/);
    assert.throws(() => checkResults({
      policy: 'success', fast: 'success', slow: result, runSlow: 'true',
    }), /Slow node/);
  }
  for (const result of ['failure', 'cancelled', 'success', '']) {
    assert.throws(() => checkResults({
      policy: 'success', fast: 'success', slow: result, runSlow: 'false',
    }), /Slow node/);
  }
  for (const runSlow of ['', 'TRUE', 'unexpected', undefined]) {
    assert.throws(() => checkResults({
      policy: 'success', fast: 'success', slow: 'skipped', runSlow,
    }), /Invalid run-slow/);
  }
});

test('real git diff includes renamed-out paths but excludes base-only changes', (t) => {
  const dir = mkdtempSync(join(tmpdir(), 'kubeedge-conformance-policy-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const git = (...args) => execFileSync('git', args, { cwd: dir, encoding: 'utf8' }).trim();
  const commit = () => {
    git('add', '.');
    git('-c', 'user.name=CI Test', '-c', 'user.email=ci-test@example.invalid',
      '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture');
    return git('rev-parse', 'HEAD');
  };
  git('init', '-q');
  mkdirSync(join(dir, 'edge'));
  mkdirSync(join(dir, 'docs'));
  writeFileSync(join(dir, 'edge', 'runtime.go'), 'original runtime fixture\n');
  const ancestor = commit();
  git('checkout', '-qb', 'pr');
  renameSync(join(dir, 'edge', 'runtime.go'), join(dir, 'docs', 'runtime.go'));
  const prHead = commit();
  git('checkout', '-qb', 'base', ancestor);
  writeFileSync(join(dir, 'edge', 'base-only.go'), 'base-only fixture\n');
  const prBase = commit();
  const files = changedFiles(prBase, prHead, (command, args, options) =>
    execFileSync(command, args, { ...options, cwd: dir }));
  assert.deepEqual(files.sort(), ['docs/runtime.go', 'edge/runtime.go']);
  assert.equal(files.some(isHighRisk), true);

  const eventPath = join(dir, 'event.json');
  const outputPath = join(dir, 'output');
  const summaryPath = join(dir, 'summary');
  const cli = join(__dirname, 'conformance-policy.cjs');
  function select(event) {
    writeFileSync(eventPath, JSON.stringify(event));
    writeFileSync(outputPath, '');
    writeFileSync(summaryPath, '');
    return spawnSync(process.execPath, [cli, 'select'], {
      cwd: dir,
      encoding: 'utf8',
      env: {
        ...process.env,
        GITHUB_EVENT_NAME: 'pull_request',
        GITHUB_EVENT_PATH: eventPath,
        GITHUB_OUTPUT: outputPath,
        GITHUB_STEP_SUMMARY: summaryPath,
      },
    });
  }
  const event = {
    pull_request: { base: { sha: prBase }, head: { sha: prHead }, labels: [] },
  };
  let result = select(event);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(outputPath, 'utf8'), 'run-slow=true\n');
  assert.match(readFileSync(summaryPath, 'utf8'), /High-risk/);

  event.pull_request.base.sha = prHead;
  result = select(event);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(outputPath, 'utf8'), 'run-slow=false\n');

  event.pull_request.labels = [{ name: 'ci/full-conformance' }];
  result = select(event);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(outputPath, 'utf8'), 'run-slow=true\n');

  event.pull_request.labels = [];
  event.pull_request.base.sha = '0'.repeat(40);
  result = select(event);
  assert.notEqual(result.status, 0);
  assert.equal(readFileSync(outputPath, 'utf8'), '');
  assert.match(result.stderr, /git diff|Invalid|bad object/);
});

test('required-check CLI propagates failures', () => {
  for (const [runSlow, slow, expected] of [
    ['true', 'success', 0],
    ['false', 'skipped', 0],
    ['true', 'skipped', 1],
    ['true', 'failure', 1],
    ['true', 'cancelled', 1],
    ['', 'skipped', 1],
  ]) {
    const result = spawnSync(process.execPath, [join(__dirname, 'conformance-policy.cjs'), 'check'], {
      encoding: 'utf8',
      env: {
        ...process.env,
        POLICY_RESULT: 'success',
        FAST_RESULT: 'success',
        SLOW_RESULT: slow,
        RUN_SLOW: runSlow,
      },
    });
    assert.equal(result.status, expected, result.stderr);
  }
});
