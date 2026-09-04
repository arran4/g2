import yaml
import sys

def run_tests():
    with open(".github/workflows/ci.yml") as f:
        ci = yaml.safe_load(f)

    jobs = ci.get('jobs', {})

    # 1. Check prepare-release-tag skipped on pushed v*
    prepare = jobs.get('prepare-release-tag')
    assert prepare, "prepare-release-tag missing"
    assert "github.event_name == 'workflow_dispatch'" in prepare.get('if', ''), "prepare-release-tag must be workflow_dispatch only"

    # 2. Check goreleaser requirements
    goreleaser = jobs.get('goreleaser')
    assert goreleaser, "goreleaser job missing"
    assert "needs.go-test.result == 'success'" in goreleaser.get('if', ''), "failed test gate must prevent publish"
    assert "needs.go-test.result == 'skipped'" not in goreleaser.get('if', ''), "skipped test gate must prevent publish"
    assert "needs.prepare-release-tag.result == 'skipped'" in goreleaser.get('if', ''), "must allow skipped prepare-release-tag"

    # 3. Check RC/Alpha are publish, test is snapshot
    steps = goreleaser.get('steps', [])
    run_goreleaser_step = next(s for s in steps if "Run GoReleaser" in s.get('name', ''))
    args = run_goreleaser_step.get('with', {}).get('args', '')

    assert "release-test" in args and "snapshot" in args, "release-test must snapshot"
    assert "test-" in args and "snapshot" in args, "test- tags must snapshot"
    assert "release-rc" not in args, "release-rc must not trigger snapshot"
    assert "release-alpha" not in args, "release-alpha must not trigger snapshot"

    # 4. Check manual tagging idempotency and correct SHA handling
    tag_step = next((s for s in steps if "Tag commit locally and push" in s.get('name', '')), None)
    assert tag_step, "tag step missing"

    # Needs to not run on release-test
    assert "inputs.mode != 'release-test'" in tag_step.get('if', ''), "manual test mode must not push a permanent tag"

    run_script = tag_step.get('run', '')
    assert "git fetch --tags" in run_script, "must fetch tags"
    assert "TAG_SHA=$(git rev-list -n 1 \"$TAG\")" in run_script, "must check tag SHA"
    assert "exit 1" in run_script, "must fail heavily on mismatched SHA"
    assert "git push origin \"$TAG\"" in run_script, "must push tag if correct"

    # 5. Check no duplicate publishing paths
    assert "publish-draft" not in jobs, "competing draft publish job remains"
    assert "promote-release" not in jobs, "competing promote job remains"

    print("✅ All static release graph checks passed!")

if __name__ == '__main__':
    run_tests()
