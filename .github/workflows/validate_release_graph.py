import yaml
import sys

def run_tests():
    with open(".github/workflows/ci.yml") as f:
        ci = yaml.safe_load(f)

    jobs = ci.get('jobs', {})

    # Check 7: Only GoReleaser owns GitHub Release publication
    assert "publish-draft" not in jobs, "competing draft publish job remains"
    assert "promote-release" not in jobs, "competing promote job remains"

    goreleaser = jobs.get('goreleaser')
    assert goreleaser, "goreleaser job missing"
    g_if = goreleaser.get('if', '')

    # Check 1: manual workflow_dispatch release-* enters goreleaser only with go-test == success
    # Check 6: failed/skipped test gates cannot publish
    assert "needs.go-test.result == 'success'" in g_if, "failed test gate must prevent publish"
    assert "needs.go-test.result == 'skipped'" not in g_if, "skipped test gate must prevent publish"

    # Check 2: externally pushed refs/tags/v* enters goreleaser with prepare-release-tag skipped
    assert "needs.prepare-release-tag.result == 'skipped'" in g_if, "must allow skipped prepare-release-tag for external tag push"
    assert "startsWith(github.ref, 'refs/tags/v')" in g_if, "must support standard v* tag pushes"

    # Check 3: externally pushed refs/tags/test-* enters goreleaser in snapshot mode
    assert "startsWith(github.ref, 'refs/tags/test-')" in g_if, "must support test-* tag pushes"

    steps = goreleaser.get('steps', [])
    run_goreleaser_step = next(s for s in steps if "Run GoReleaser" in s.get('name', ''))
    args = run_goreleaser_step.get('with', {}).get('args', '')

    assert "contains(github.ref, 'test-')" in args or "startsWith(github.ref, 'refs/tags/test-')" in args, "test- tags must trigger snapshot mode"
    assert "snapshot" in args, "snapshot argument must be conditionally present"

    # Check 4: manual release-test is snapshot-only and does not run the permanent tag-push step
    assert "inputs.mode == 'release-test'" in args and "snapshot" in args, "manual release-test must trigger snapshot"

    tag_step = next((s for s in steps if "Tag commit locally and push" in s.get('name', '')), None)
    assert tag_step, "tag step missing"
    assert "inputs.mode != 'release-test'" in tag_step.get('if', ''), "manual test mode must not push a permanent tag"

    # Check 5: manual release-rc / release-alpha are non-snapshot publishing paths
    assert "release-rc" not in args, "release-rc must not trigger snapshot mode"
    assert "release-alpha" not in args, "release-alpha must not trigger snapshot mode"

    # Check 8: manual permanent tag creation remains retry-safe and SHA-checked
    run_script = tag_step.get('run', '')
    assert "git fetch --tags" in run_script, "must fetch tags"
    assert "TAG_SHA=$(git rev-list -n 1 \"$TAG\")" in run_script, "must check tag SHA"
    assert "exit 1" in run_script, "must fail heavily on mismatched SHA"
    assert "git push origin \"$TAG\"" in run_script, "must push tag if correct"

    print("✅ All static release graph checks passed!")

if __name__ == '__main__':
    run_tests()
