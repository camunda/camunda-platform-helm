{{/* vim: set filetype=mustache: */}}

{{/*
********************************************************************************
Camunda Hub helpers.

The camundaHub component consolidates Console and WebModeler into a single
logical unit. The backward-compatibility shim helpers that bridge the legacy
console.* / webModeler.* keys to the new camundaHub.* key live in
templates/common/_helpers.tpl (see "camundaHub.consoleEnabled",
"camundaHub.webModelerEnabled", "camundaHub.consoleValues",
"camundaHub.webModelerValues").

This file is reserved for any camundaHub-specific helpers that do NOT belong
in the common shim layer. For now, no additional helpers are needed because:
  - Console and WebModeler templates continue to use their own _helpers.tpl
  - Resource names are intentionally preserved for smooth 8.9 → 8.10 upgrade
  - The shim helpers in common/_helpers.tpl handle the enabled-check and
    value-merge logic
********************************************************************************
*/}}
