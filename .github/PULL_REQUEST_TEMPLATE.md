# Pull Request Title

<!--
Provide a clear, concise title that describes the change.
Example: "Add round-robin fallback routing strategy"
-->

## Description

<!--
Explain what this PR does and why.

Focus on:
- What problem it solves for rigrun (local-first, LLM routing, providers, configs, etc.)
- High-level approach and any key trade-offs
-->

## Type of change

<!--
Select all that apply.
-->

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Refactor (non-functional change, code structure/cleanup)
- [ ] Performance improvement
- [ ] Documentation update
- [ ] Tests (adding or improving tests only)
- [ ] CI/CD or tooling
- [ ] Chore / maintenance
- [ ] Other (describe below)

## Related issues

<!--
Link any relevant issues, discussions, or PRs.

Examples:
- Closes #123
- Related to #456
-->

- Closes #
- Related to #

## Testing done

<!--
Describe how you tested this change.

Consider:
- Unit tests
- Integration / end-to-end tests
- Manual testing (describe scenarios)
- Different LLM providers, models, or routing configurations
-->

- [ ] Unit tests:
- [ ] Integration / E2E tests:
- [ ] Manual testing:

Commands run (if applicable):

```bash
# examples:
# cargo test
# npm test
# pytest
```

## Checklist

<!--
Confirm that these items are complete before requesting review.
-->

- [ ] Code compiles and runs locally
- [ ] New and existing tests pass locally
- [ ] Tests were added or updated to cover this change (if applicable)
- [ ] Code is formatted and linted (e.g., `fmt`, `lint`, or equivalent)
- [ ] Public APIs and configuration options are documented
- [ ] Documentation updated (README, docs site, comments) as needed
- [ ] Examples, sample configs, or usage snippets updated (if applicable)
- [ ] No secrets or sensitive data are added to the repo
- [ ] For user-facing or API changes, behavior is clearly described in the Description section

## Breaking changes

<!--
If this change breaks existing behavior, document it clearly.

Include:
- What breaks
- Who is affected (users, integrators, operators)
- Migration steps and examples
-->

- [ ] This PR introduces breaking changes

If yes, describe:

1. **What changed:**
2. **Impact:**
3. **Migration steps:**

```diff
# Example migration snippet (config, API, CLI, etc.)
```
