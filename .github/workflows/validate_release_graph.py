import yaml
import sys

def run_tests():
    with open(".github/workflows/ci.yml") as f:
        ci = yaml.safe_load(f)

    jobs = ci.get('jobs', {})

    route = jobs.get('route')
    assert route, "route job missing"

    on_triggers = ci.get(True, {}) if True in ci else ci.get('on', {})

    inputs_options = []
    if 'workflow_dispatch' in on_triggers:
        inputs_options = on_triggers['workflow_dispatch'].get('inputs', {}).get('mode', {}).get('options', [])
    assert 'publish-tag' in inputs_options, "publish-tag missing from inputs mode options"

    prepare = jobs.get('prepare-release-tag')
    assert prepare, "prepare-release-tag missing"
    assert "needs.route.outputs.run_release == 'true'" in prepare.get('if', ''), "prepare-release-tag must require run_release == 'true'"

    validation = jobs.get('release-validation')
    assert validation, "release-validation job missing"
    assert "needs.route.outputs.run_release == 'true'" in validation.get('if', ''), "release-validation must require run_release == 'true'"

    context = jobs.get('release-context')
    assert context, "release-context job missing"
    assert "needs.route.outputs.run_release == 'true'" in context.get('if', ''), "release-context must require run_release == 'true'"

    goreleaser = jobs.get('goreleaser')
    assert goreleaser, "goreleaser job missing"
    g_if = goreleaser.get('if', '')

    assert "needs.route.outputs.run_release == 'true'" in g_if, "goreleaser must require run_release == 'true'"
    assert "github.event_name == 'push'" in g_if, "goreleaser must support push"
    assert "inputs.mode == 'publish-tag'" in g_if, "goreleaser must support publish-tag mode"
    assert "release-context" in goreleaser.get('needs', []), "goreleaser must depend on release-context"

    # Check 7: Only GoReleaser owns GitHub Release publication
    assert "publish-draft" not in jobs, "competing draft publish job remains"
    assert "promote-release" not in jobs, "competing promote job remains"

    # Check 3: test-* tag pushes use snapshot
    assert "(contains(needs.release-context.outputs.release_tag, 'test-') || (github.event_name == 'workflow_dispatch' && inputs.mode == 'release-test')) && '--snapshot' || ''" in goreleaser.get('steps', [{}])[2].get('with', {}).get('args', ''), "test- tags must trigger snapshot mode"

    # Check 4: manual release-test is snapshot-only and does not run permanent tag-push step
    context_script = context.get('steps', [{}])[1].get('run', '')
    assert "if [[ \"$INPUT_MODE\" != \"release-test\" ]]; then" in context_script, "manual release-test must not push permanent tag"
    assert "gh workflow run" in context_script, "must explicitly dispatch publish-tag"


    # Check 1: Manual release-* restricted to exact origin/main
    prep_steps = prepare.get('steps', [])
    exact_main = [s for s in prep_steps if "Verify Exact Origin/Main" in s.get('name', '')]
    assert exact_main, "Must check exact main commit before tagging"

    # Check 2: Release concurrency cancel-in-progress uses safe release semantics
    concurrency = ci.get('concurrency', {})
    assert "startsWith(github.event.inputs.mode, 'release-')" in str(concurrency.get('cancel-in-progress', '')), "cancel-in-progress must be safe for release prep"

    # Check 4: Annotated tag recovery
    tag_run = [s for s in prep_steps if s.get('id') == 'tag']
    if tag_run:
        assert "^{}" in tag_run[0].get('run', ''), "Must safely check peeled annotated tags in prepare-release-tag"

    # Check 5: Permissions are read-only at top level
    assert ci.get('permissions', {}).get('contents') == 'read', "Top-level permissions must be read-only by default"

    print("✅ All static release graph checks passed!")

if __name__ == '__main__':
    run_tests()
