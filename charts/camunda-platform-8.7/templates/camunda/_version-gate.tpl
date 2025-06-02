# templates/_version-gate.tpl
{{- /*
     Abort the release if the new image tag is lower than the one
     already running. Works only on upgrades; does nothing on an
     initial install because lookup returns nil.

     ⚠️ Helm Name Scoping & Lookup Note:
     -----------------------------------
     When using Helm's `lookup` function to fetch existing resources, the resource name must be constructed
     in the exact same way as it was during the original deployment. This is typically done by combining
     the release name (e.g., `$ctx.Release.Name`) with a resource-specific suffix (e.g., `"zeebe-gateway"`).

     If you pass a fully rendered name (e.g., via `include "zeebe.names.gateway" .`) from a different context,
     it may not match the actual resource name in the cluster if the context (such as release name, namespace,
     or chart scope) differs. This can cause lookups to fail, even if the printed name appears correct.

     **Best Practice:**  
     Always generate the resource name for lookups inside the helper using the current release context,
     or pass the context needed to reconstruct the name, not just the rendered string. This ensures
     the lookup will always target the correct resource, regardless of where or how the helper is called.

     Edge Cases Handled:

     1. Initial Installation
        - When no existing deployment is found or Release.IsInstall is true
        - WHY: First deployments should always succeed as there's no version to compare against
        - HOW: Returns early when lookup returns nil or on initial install

     2. Special Tag "latest"
        - When either current or new tag is "latest"
        - WHY: "latest" is not a version and can't be compared semantically
        - HOW: Skips version check when "latest" is involved to allow deployment

     3. Version Prefix Handling
        - Tags with or without "v" prefix (e.g., "v1.0.0" vs "1.0.0")
        - WHY: Both formats are common in Docker tags but need normalized comparison
        - HOW: Strips "v" prefix before comparison

     4. Tag Format
        - Format: [a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}
        - WHY: Must comply with OCI image spec tag format
        - HOW: Validates using regex pattern

     5. Invalid Semver
        - Non-semver compliant version strings
        - WHY: Only semver allows reliable version comparison
        - HOW: Validates format before comparison, fails if invalid

     6. Missing Container/Image
        - When deployment exists but container/image details are missing
        - WHY: Corrupted or invalid deployments should not proceed
        - HOW: Explicit checks with descriptive error messages

     7. Parameter Validation
        - Missing or invalid input parameters
        - WHY: Template requires specific inputs to function correctly
        - HOW: Early validation with clear error messages

     8. Tag Precedence
        - $global.image.tag takes precedence over $deployment.image.tag
        - WHY: Allows global version override at chart level
        - HOW: Checks $global.image.tag first, falls back to $deployment.image.tag
*/ -}}

{{- define "camunda.version-gate" -}}
{{- $ctx := .ctx }}
{{- if $ctx.Release.IsUpgrade }}
  {{- /* This template is used to prevent downgrades of the Camunda Platform */ -}}
  {{- $global := $ctx.Values.global -}}
  {{- $deployment := .deployment -}}
  {{- $name := .name -}}
  
  {{- /* Input validation */ -}}
  {{- if not $global }}
    {{- fail "global parameter must be provided. This is the global of the chart." -}}
  {{- end -}}
  {{- if not $deployment }}
    {{- fail "deployment parameter must be provided. This is the deployment to check the version for." -}}
  {{- end -}}
  {{- if not $name }}
    {{- fail "name parameter must be provided. This is the name of the service to check the version for." -}}
  {{- end -}}

  {{- /* Determine the new tag to use, with $global.image.tag taking precedence */ -}}
  {{- $newTag := "" -}}
  {{- if and $global.image $global.image.tag -}}
    {{- $newTag = toString $global.image.tag -}}
  {{- else if $deployment.image.tag -}}
    {{- $newTag = toString $deployment.image.tag -}}
  {{- else -}}
    {{- fail "neither global.image.tag nor deployment.image.tag is set" -}}
  {{- end -}}

  {{- /* fail (printf "%s-%s" $ctx.Release.Name $name) | nindent 0  */ -}}
  {{- /* $current := lookup "apps/v1" "Deployment" $ctx.Release.Namespace (printf "%s-%s" $ctx.Release.Name $name) -}} */ -}}
  {{- /* fail (printf $name) | nindent 0 -}} */ -}}
  {{- $current := lookup "apps/v1" "Deployment" $ctx.Release.Namespace (include $name .nameContext) -}}
    
  {{- if $current }}
    {{- /* Extract current version from deployment */ -}}
    {{- $container := index $current.spec.template.spec.containers 0 -}}
    
    {{- if not $container }}
      {{- fail "no container found in current deployment" -}}
    {{- end -}}
    {{- $img := $container.image | toString -}}
    {{- if not $img }}
      {{- fail "no image found in current deployment" -}}
    {{- end -}}

    {{- /* Extract current tag */ -}}
    {{- $curTag := "" -}}
    {{- if contains ":" $img }}
      {{- $curTag = last (splitList ":" $img) -}}
    {{- else -}}
      {{- $curTag = "latest" -}}
    {{- end -}}

    {{- /* Handle digest format */ -}}
    {{- if contains "@sha256:" $curTag }}
      {{- fail "cannot compare versions when using image digest format" -}}
    {{- end -}}

    {{- /* 
    Validate tag format: [a-zA-Z0-9_][a-zA-Z0-9._-]{0,127} 
    https://github.com/opencontainers/distribution-spec/blob/main/spec.md#workflow-categories
    */ -}}
    {{- $tagRegex := "^[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}$" -}}
    {{- if not (regexMatch $tagRegex $curTag) }}
      {{- fail (printf "invalid tag format in current image: %q" $curTag) -}}
    {{- end -}}
    {{- if not (regexMatch $tagRegex $newTag) }}
      {{- fail (printf "invalid tag format in new image: %q" $newTag) -}}
    {{- end -}}

    {{- /* Handle special tags */ -}}
    {{- if not (or (eq $curTag "latest") (eq $newTag "latest")) }}
      {{- /* 
      Validate semver format for version comparison
      https://semver.org/
      */ -}}
      {{- $semverRegex := "^v?\\d+\\.\\d+\\.\\d+(-[0-9A-Za-z-.]+)?(\\+[0-9A-Za-z-.]+)?$" -}}
      {{- if not (regexMatch $semverRegex $curTag) }}
        {{- fail (printf "current tag %q is not a valid semver" $curTag) -}}
      {{- end -}}
      {{- if not (regexMatch $semverRegex $newTag) }}
        {{- fail (printf "new tag %q is not a valid semver" $newTag) -}}
      {{- end -}}

      {{- /* Normalize version strings by removing 'v' prefix if present */ -}}
      {{- $curTag = regexReplaceAll "^v" $curTag "" -}}
      {{- $newTag = regexReplaceAll "^v" $newTag "" -}}

      {{- /* Refuse downgrade */ -}}
      {{- if not (semverCompare (printf ">= %s" $curTag) $newTag) -}}
        {{- fail (printf "downgrade detected: %s -> %s" $curTag $newTag) -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}
