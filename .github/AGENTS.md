
## PR instructions

- Title format: <type>[optional scope]: <description>
- Commit format: <type>[optional scope]: <description>
- valid types:
    feat
    fix
    refactor
    revert
    test
    docs
    style
    build
    ci
    cd
    chore
    chore(deps)
    chore(release)
    deps

## Development tips

- Complex logic for CI pipelines (>20 lines) should be implemented as golang scripts inside the scripts directory and then called with github actions. Do not implement this in bash.
- When writing any golang, the scripts must have unit tests

