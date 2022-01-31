## Contributing

[fork]: /fork
[pr]: /compare
[CODE_OF_CONDUCT]: CODE_OF_CONDUCT.MD

Hi there! We're thrilled that you'd like to contribute to this project. Your help is essential for keeping it great.

Please note that this project is released with a [Contributor Code of Conduct](https://github.com/camunda-community-hub/camunda-cloud-helm/blob/main/CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

## Issues and PRs

If you have suggestions for how this project could be improved, or want to report a bug, open an issue! We'd love all and any contributions. If you have questions, too, we'd love to hear them.

We'd also love PRs. If you're thinking of a large PR, we advise opening up an issue first to talk about it, though! Look at the links below if you're not sure how to open a PR.

## Submitting a pull request

Please feel free to fork this repository and open a pull request to update this documentation.

Make sure that your provided PR's works via:

 * `helm lint` to run the linting
 * `helm template <releasename> chartPath/` to generate the template files
 * `helm install <releasename> chartPath/` to install a helm release in your k8 cluster (e.g. kind)

### Best Practices

Make sure you're familiar with some helm best practices like:

 * https://helm.sh/docs/chart_best_practices/
 * https://codersociety.com/blog/articles/helm-best-practices

### Commit Guidelines

Commit messages should follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) format.

For example:

```
fix: set correct port

Previously no metrics have been exposed because the wrong port was used. This commit fixes the post and set it to 9600.
```

Available commit types:

* `feat` - enhancements, new features
* `fix` - bug fixes
* `refactor` - non-behavior changes
* `test` - only changes in tests
* `docs` - changes in the documentation, readme, etc.
* `style` - apply code styles
* `build` - changes to the build (e.g. to Maven's `Chart.yaml`)
* `ci` - changes to the CI (e.g. to GitHub related configs)


### Documentation

The `values.yaml` file follows helm best practices https://helm.sh/docs/chart_best_practices/values/

This means:
  * Variable names should begin with a lowercase letter, and words should be separated with camelcase.
  * Every defined property in values.yaml should be documented. The documentation string should begin with the name of the property that it describes, and then give at least a one-sentence description

Furthermore, we try to apply the following pattern: `# [VarName] [conjunction] [definition]`

_VarName:_

  * In the documentation the variable name is started with a big letter, similar to kubernetes resource documentation.
  * If the variable is part of a subsection/object we use a json path expression (to make it more clear where the variable belongs to).
    The root (chart name) is omitted (e.g. zeebe). This is useful for using --set in helm.

_Conjunction:_

  * [defines] for mandatory configuration
  * [can be used] for optional configuration
  * [if true] for toggles
  * [configuration] for section/group of variables


## Resources

- [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/)
- [Using Pull Requests](https://help.github.com/articles/about-pull-requests/)
- [GitHub Help](https://help.github.com)
- [CODE_OF_CONDUCT](https://github.com/camunda-community-hub/community/blob/main/CODE_OF_CONDUCT.MD)
