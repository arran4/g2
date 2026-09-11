1. **Remove deprecated workflows**:
   - Delete `.github/workflows/delete-all-cache.yml`
   - Delete `.github/workflows/regenerate-all-cache.yml`
   These are legacy competing workflows, and we're moving to idempotent read-only check and an explicit repair.

2. **Add `cache-consistency.yml`**:
   - Creates a scheduled GitHub Action that runs `g2 cache verify .`
   - Non-destructive (no commits/push).
   - Fails the workflow if drift is detected.

3. **Add `repair-cache.yml`**:
   - Triggered manually (workflow_dispatch).
   - Runs `g2 cache reconcile .` (idempotently generates/removes cache entries).
   - Creates a PR via `peter-evans/create-pull-request` with the repair changes (no direct push to main).

4. **Update `scheduled-daily-tasks.yml`**:
   - Remove the `run_egencache` input and the `Run egencache update` step.
   - Remove the `Snapshot manifests before dedupe` and `Restore manifests after dedupe` steps around the `Remove duplicate ebuilds` step, since `g2` is now safe to run directly.
   - Remove the safety guard assertions checking for manifest diffs.

5. **Update Layout and Workflow Files**:
   - Modify `metadata/layout.conf` to remove `md5-dict` and use `md5-cache` instead.
   - Replace any lingering references to `md5-dict` in all YAML update workflow files.

6. **Pre-commit Instructions**:
   - Get instructions from the tool `pre_commit_instructions` and follow them to check that my work is correct and does not violate policies.

7. **Submit**:
   - Push to a new branch, request review.
