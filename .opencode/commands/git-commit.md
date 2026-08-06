---
description: Inspect Git changes, produce a commit message, and commit them
---

# Git Commit Skill

Use this skill when the user asks to review and commit the current repository changes.

## Workflow

1. Confirm the current directory is a Git repository.
2. Inspect the repository state:
   ```bash
   git status --short
   git branch --show-current
   git diff --stat
   git diff
   git diff --cached
   ```
3. Review relevant changed files when the diff alone does not provide enough context.
4. Understand:
   * what was added, changed, removed, or renamed;
   * the purpose of the changes;
   * whether the changes form one coherent commit;
   * whether generated, temporary, secret, or unrelated files are present.
5. Do not modify source files unless the user explicitly requests changes.
6. Do not include secrets, credentials, local databases, build outputs, temporary files, or unrelated changes in the commit.
7. Stage the intended changes:
   ```bash
   git add <files>
   ```
8. Verify the staged content:
   ```bash
   git diff --cached --stat
   git diff --cached
   ```
9. Create a commit message containing:
   * a concise imperative title;
   * an itemized summary of the main changes.

## Commit message format

```text
<imperative title, preferably 72 characters or fewer>

- <first significant change>
- <second significant change>
- <additional relevant change>
```

Example:

```text
<somePrefix if necessary> - Improve research workspace navigation

- Add persistent search, revision, plan, and run selectors
- Preserve navigation state through URL query parameters
- Add empty and loading states for run-scoped pages
```

## Commit rules

* Base the message only on the staged diff.
* Use an imperative title such as `Add`, `Update`, `Fix`, `Refactor`, or `Remove`.
* Describe the purpose of the changes, not every modified line.
* Keep summary items concise and specific.
* Do not use vague titles such as `Update files`, `Changes`, or `Fix stuff`.
* Do not amend an existing commit unless explicitly requested.
* Do not push the commit unless explicitly requested.
* Do not bypass hooks with `--no-verify`.
* If a hook fails, report the failure and do not claim the commit succeeded.
* If there are no relevant changes, do not create an empty commit.

## Final verification

After committing, run:

```bash
git status --short
git log -1 --format='%h %s'
```

Report:

* the commit hash;
* the commit title;
* the itemized summary;
* any changes that remain uncommitted.