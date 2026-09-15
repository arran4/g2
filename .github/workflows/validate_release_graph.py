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

    route_steps = route.get('steps', [])
    route_step = [s for s in route_steps if s.get('id') == 'route']
    assert route_step, "route step missing"
    route_script = route_step[0].get('run', '')

    publish_tag_marker = 'elif [[ "$mode" == "publish-tag" ]]; then'
    assert publish_tag_marker in route_script, "publish-tag route missing"
    publish_tag_block = route_script.split(publish_tag_marker, 1)[1].split('elif [[ "$mode" == release-* ]]', 1)[0]
    assert 'run_publisher=true' in publish_tag_block, "publish-tag must route to publisher"
    assert 'run_code_checks=false' in publish_tag_block, "publish-tag must not rerun code checks"
    assert 'run_build=false' in publish_tag_block, "publish-tag must not rerun build checks"

    tag_push_marker = 'elif [[ "$EVENT_NAME" == "push" && "$REF_TYPE" == "tag"'
    assert tag_push_marker in route_script, "eligible external tag-push route missing"
    tag_push_block = route_script.split(tag_push_marker, 1)[1].split('elif [[ "$EVENT_NAME" == "release" ]]', 1)[0]
    assert 'run_publisher=true' in tag_push_block, "eligible external tag pushes must route to publisher"
    assert 'run_code_checks=false' in tag_push_block, "eligible external tag pushes must not rerun code checks"
    assert 'run_build=false' in tag_push_block, "eligible external tag pushes must not rerun build checks"

    prepare = jobs.get('prepare-release-tag')
    assert prepare, "prepare-release-tag missing"
    assert "needs.route.outputs.run_release == 'true'" in prepare.get('if', ''), "prepare-release-tag must require run_release == 'true'"

    validation = jobs.get('release-ready')
    assert validation, "release-ready job missing"

    route_outputs = route.get('outputs', {})
    assert 'is_nightly' not in route_outputs, "route is_nightly should be removed"
    assert 'is_monthly' not in route_outputs, "route is_monthly should be removed"
    with open(".github/workflows/ci.yml") as ci_f:
        ci_str = ci_f.read()
    assert 'is_nightly' not in ci_str, "no job should reference is_nightly"
    assert 'is_monthly' not in ci_str, "no job should reference is_monthly"
    assert 'EVENT_NAME\" == \"release\"' in ci_str, "release no-op should be configured in route"
    assert 'PEELED_SHA=$(git ls-remote --tags origin "refs/tags/$TAG^{}"' in ci_str, "push fallback must include annotated-tag peeling"


    autofix = jobs.get('autofix')
    go_fmt_pr = jobs.get('go-fmt-pr')
    assert autofix and not go_fmt_pr, "Autofix lane must exist and go-fmt-pr must be removed to avoid competing PRs"

    assert "needs.route.outputs.run_autofix == 'true'" in autofix.get('if', ''), "autofix lane must use run_autofix output"

    assert 'git fetch --tags --force' not in str(prepare), "prepare-release-tag must not contain stale logic"


    goreleaser = jobs.get('goreleaser') or jobs.get('publisher')
    assert goreleaser, "goreleaser/publisher job missing"
    g_if = goreleaser.get('if', '')

    assert "needs.route.outputs.run_publisher == 'true'" in g_if, "goreleaser must require run_publisher == 'true'"
    assert "needs.route.outputs.run_release == 'true'" not in g_if, "publisher must not be reachable directly from release preparation"
    assert "release-ready" in goreleaser.get('needs', []), "goreleaser must depend on release-ready"

    # Check 7: Only GoReleaser owns GitHub Release publication
    assert "publish-draft" not in jobs, "competing draft publish job remains"
    assert "promote-release" not in jobs, "competing promote job remains"

    # Check 3: test-* and *-test* tag pushes use snapshot
    steps = goreleaser.get('steps', [])
    args_step = [s for s in steps if s.get('id') == 'args']
    assert args_step, "goreleaser args step missing"
    assert "snapshot" in args_step[0].get('run', ''), "test tags must trigger snapshot mode"

    # Check 4: manual explicit dispatch
    prep_steps = prepare.get('steps', [])
    dispatch_step = [s for s in prep_steps if "Dispatch Publisher" in s.get('name', '')]
    assert dispatch_step, "must explicitly dispatch publish-tag"
    assert "gh workflow run ci.yml" in dispatch_step[0].get('run', ''), "must explicitly dispatch publish-tag"


    # Check 1: Manual release-* restricted to exact origin/main
    exact_main = [s for s in prep_steps if "Verify Exact Origin/Main" in s.get('name', '')]
    assert exact_main, "Must check exact main commit before tagging"
    exact_main_script = exact_main[0].get('run', '')
    assert "refs/heads/main" in exact_main_script, "Must ensure current ref is refs/heads/main"
    assert "git fetch origin main" in exact_main_script, "Must fetch origin/main"
    assert "$MAIN_SHA\" != \"$GITHUB_SHA" in exact_main_script, "Must require exact commit equality with origin/main"

    # Check 1b: Final race check
    tag_push_step = [s for s in prep_steps if "Tag and push" in s.get('name', '')]
    assert tag_push_step, "Must have Tag and push step"
    tag_push_script = tag_push_step[0].get('run', '')
    assert "CURRENT_MAIN_SHA=$(git rev-parse origin/main)" in tag_push_script, "Must check race condition before tagging"
    assert "$CURRENT_MAIN_SHA\" != \"$GITHUB_SHA" in tag_push_script, "Must fail race condition check if origin/main advanced"

    # Check 2: Release concurrency cancel-in-progress uses safe release semantics
    concurrency = ci.get('concurrency', {})
    assert "startsWith(github.event.inputs.mode, 'release-')" in str(concurrency.get('cancel-in-progress', '')), "cancel-in-progress must be safe for release prep"

    # Check 4: Annotated tag recovery
    assert "^{}" in tag_push_script, "Must safely check peeled annotated tags in prepare-release-tag"

    # Check 5: Permissions are read-only at top level
    assert ci.get('permissions', {}).get('contents') == 'read', "Top-level permissions must be read-only by default"

    print("✅ All static release graph checks passed!")

if __name__ == '__main__':
    run_tests()
