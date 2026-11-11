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

const { execFileSync } = require('node:child_process');
const { readFileSync, appendFileSync } = require('node:fs');

const fullConformanceLabel = 'ci/full-conformance';
const highRiskPrefixes = [
  'edge/',
  'common/',
  'pkg/',
  'cloud/pkg/edgecontroller/',
  'cloud/pkg/cloudhub/',
  'vendor/',
  'staging/',
  'build/',
  'hack/',
  'tests/',
  '.github/',
];

function isHighRisk(path) {
  return highRiskPrefixes.some((prefix) => path.startsWith(prefix)) ||
    /(^|\/)(go\.(mod|sum|work|work\.sum)|Makefile|[^/]*\.mk)$/.test(path);
}

function changedFiles(base, head, exec = execFileSync) {
  if (![base, head].every((sha) => typeof sha === 'string' && /^[a-f0-9]{40}$/i.test(sha))) {
    throw new Error('Missing or invalid PR base/head SHA');
  }
  // Disable rename detection so moving code out of a sensitive path still triggers coverage.
  const output = exec('git', [
    'diff', '--no-ext-diff', '--no-renames', '--name-only', '-z', `${base}...${head}`, '--',
  ], { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
  return output.split('\0').filter((path) => path.length > 0);
}

function selectCoverage(eventName, event, getChangedFiles = changedFiles) {
  if (['push', 'schedule', 'workflow_dispatch'].includes(eventName)) {
    return { runSlow: true, reason: 'Full coverage for post-merge, daily or manual runs' };
  }
  if (eventName !== 'pull_request' || !event.pull_request ||
      !Array.isArray(event.pull_request.labels)) {
    throw new Error('Unsupported event or incomplete pull request metadata');
  }
  const pr = event.pull_request;
  if (pr.labels.some((label) => label.name === fullConformanceLabel)) {
    return { runSlow: true, reason: 'Full conformance label is present' };
  }
  const files = getChangedFiles(pr.base?.sha, pr.head?.sha);
  if (files.some(isHighRisk)) {
    return { runSlow: true, reason: 'High-risk code, dependency or CI paths changed' };
  }
  return { runSlow: false, reason: 'Ordinary PR: back-off tests deferred to post-merge and daily CI' };
}

function checkResults({ policy, fast, slow, runSlow }) {
  if (policy !== 'success') {
    throw new Error(`Coverage policy did not succeed: ${policy}`);
  }
  if (!['true', 'false'].includes(runSlow)) {
    throw new Error(`Invalid run-slow output: ${runSlow}`);
  }
  if (fast !== 'success') {
    throw new Error(`Conformance and fast node coverage did not succeed: ${fast}`);
  }
  const expectedSlow = runSlow === 'true' ? 'success' : 'skipped';
  if (slow !== expectedSlow) {
    throw new Error(`Slow node coverage must be ${expectedSlow}, got ${slow}`);
  }
}

if (require.main === module) {
  try {
    if (process.argv[2] === 'select') {
      const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
      const result = selectCoverage(process.env.GITHUB_EVENT_NAME, event);
      appendFileSync(process.env.GITHUB_OUTPUT, `run-slow=${result.runSlow}\n`);
      appendFileSync(process.env.GITHUB_STEP_SUMMARY,
        `### Conformance coverage\n\n${result.reason}.\n\nRun slow node tests: **${result.runSlow}**\n`);
      console.log(result.reason);
    } else if (process.argv[2] === 'check') {
      checkResults({
        policy: process.env.POLICY_RESULT,
        fast: process.env.FAST_RESULT,
        slow: process.env.SLOW_RESULT,
        runSlow: process.env.RUN_SLOW,
      });
      console.log('All required conformance coverage completed successfully');
    } else {
      throw new Error('Expected select or check command');
    }
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}

module.exports = { changedFiles, checkResults, isHighRisk, selectCoverage };
